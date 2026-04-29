package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"drova/services/api-gateway/grpc_clients"
	"drova/shared/contracts"
	"drova/shared/messaging"
	pb "drova/shared/proto/driver"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	wsPingInterval = 25 * time.Second
	wsPongWait     = 35 * time.Second
)

var riderConnManager = messaging.NewConnectionManager()
var driverConnManager = messaging.NewConnectionManager()

func handleRidersWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	claims, err := parseGatewayToken(tokenStr)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if isBlacklisted(r.Context(), claims.ID) {
		http.Error(w, "token revoked", http.StatusUnauthorized)
		return
	}

	userID := r.URL.Query().Get("userID")
	if userID == "" || userID != fmt.Sprintf("%d", claims.UserID) {
		http.Error(w, "userID mismatch", http.StatusUnauthorized)
		return
	}

	conn, err := riderConnManager.Upgrade(w, r)
	if err != nil {
		appLog.Warnw("rider ws upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	riderConnManager.Add(userID, conn)
	defer riderConnManager.Remove(userID)

	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			case <-r.Context().Done():
				return
			}
		}
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var msg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(message, &msg); err != nil {
			continue
		}

		switch msg.Type {
		case contracts.TripCmdCancel:
			var cancelData struct {
				TripID   string `json:"trip_id"`
				DriverID string `json:"driver_id"`
			}
			if err := json.Unmarshal(msg.Data, &cancelData); err != nil {
				appLog.Warnw("unmarshal cancel data", zap.Error(err))
				continue
			}
			cancelEvent := messaging.TripCancelledEvent{
				TripID:   cancelData.TripID,
				RiderID:  userID,
				DriverID: cancelData.DriverID,
			}
			data, _ := json.Marshal(cancelEvent)
			ownerID := cancelData.DriverID
			if ownerID == "" {
				ownerID = userID
			}
			kmsg := messaging.KafkaMessage{
				Type:    contracts.TripEventCancelled,
				OwnerID: ownerID,
				Data:    data,
			}
			payload, _ := json.Marshal(kmsg)
			if err := kafkaClient.PublishMessage(r.Context(), messaging.TopicTripCancelled, payload); err != nil {
				appLog.Errorw("publish trip cancel", zap.Error(err))
			}
			appLog.Infow("trip cancelled", "rider", userID, "trip", cancelData.TripID)
		}
	}
}

func handleDriversWebSocket(w http.ResponseWriter, r *http.Request) {
	tokenStr := r.URL.Query().Get("token")
	claims, err := parseGatewayToken(tokenStr)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if isBlacklisted(r.Context(), claims.ID) {
		http.Error(w, "token revoked", http.StatusUnauthorized)
		return
	}

	userID := r.URL.Query().Get("userID")
	if userID == "" || userID != fmt.Sprintf("%d", claims.UserID) {
		http.Error(w, "userID mismatch", http.StatusUnauthorized)
		return
	}

	packageSlug := r.URL.Query().Get("packageSlug")
	if packageSlug == "" {
		http.Error(w, "packageSlug required", http.StatusBadRequest)
		return
	}

	conn, err := driverConnManager.Upgrade(w, r)
	if err != nil {
		appLog.Warnw("driver ws upgrade failed", zap.Error(err))
		return
	}
	defer conn.Close()

	conn.SetReadDeadline(time.Now().Add(wsPongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(wsPongWait))
		return nil
	})

	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second)); err != nil {
					return
				}
			case <-r.Context().Done():
				return
			}
		}
	}()

	driverName := r.URL.Query().Get("name")

	ctx := r.Context()

	defer func() {
		grpc_clients.DriverClient.UnregisterDriver(ctx, &pb.RegisterDriverRequest{
			DriverID:    userID,
			PackageSlug: packageSlug,
		})
	}()

	var driverData *pb.RegisterDriverResponse
	_, regErr := grpc_clients.DriverBreaker.Execute(func() (interface{}, error) {
		d, err := grpc_clients.DriverClient.RegisterDriver(ctx, &pb.RegisterDriverRequest{
			DriverID:    userID,
			PackageSlug: packageSlug,
			Name:        driverName,
		})
		if err != nil {
			return nil, err
		}
		driverData = d
		return nil, nil
	})
	if regErr != nil {
		appLog.Errorw("register driver", zap.Error(regErr))
		return
	}

	if avatarURL := r.URL.Query().Get("avatarUrl"); avatarURL != "" && driverData.Driver != nil {
		driverData.Driver.ProfilePicture = avatarURL
	}

	// Each WS connection needs its own bidirectional stream for location updates.
	// The shared connection is reused; the stream itself is per-driver-session.
	locationStream, err := grpc_clients.DriverClient.StreamLocation(ctx)
	if err != nil {
		appLog.Warnw("location stream open failed", "driver", userID, zap.Error(err))
		locationStream = nil
	} else {
		appLog.Infow("location stream opened", "driver", userID)
	}
	defer func() {
		if locationStream != nil {
			locationStream.CloseSend()
			appLog.Infow("location stream closed", "driver", userID)
		}
	}()

	driverConnManager.Add(userID, conn)
	defer driverConnManager.Remove(userID)

	if err := driverConnManager.SendMessage(userID, contracts.WSMessage{
		Type: contracts.DriverCmdRegister,
		Data: driverData.Driver,
	}); err != nil {
		appLog.Errorw("send register message", zap.Error(err))
		return
	}

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		var driverMsg struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(message, &driverMsg); err != nil {
			appLog.Warnw("unmarshal driver message", zap.Error(err))
			continue
		}

		switch driverMsg.Type {
		case contracts.DriverCmdLocation:
			var locData messaging.DriverLocationData
			if err := json.Unmarshal(driverMsg.Data, &locData); err != nil {
				appLog.Warnw("unmarshal location", zap.Error(err))
				continue
			}
			appLog.Debugw("location update", "driver", userID, "lat", locData.Lat, "lng", locData.Lng, "rider", locData.RiderID)
			if locationStream != nil {
				if err := locationStream.Send(&pb.LocationUpdate{
					DriverId:  userID,
					Latitude:  locData.Lat,
					Longitude: locData.Lng,
				}); err != nil {
					appLog.Warnw("location stream send", zap.Error(err))
				}
			}
			if locData.RiderID != "" {
				data, err := json.Marshal(locData)
				if err != nil {
					appLog.Warnw("marshal location", zap.Error(err))
					continue
				}
				payload, err := json.Marshal(messaging.KafkaMessage{
					Type:    contracts.DriverCmdLocation,
					OwnerID: locData.RiderID,
					Data:    data,
				})
				if err != nil {
					appLog.Warnw("marshal location payload", zap.Error(err))
					continue
				}
				if err := kafkaClient.PublishMessage(r.Context(), messaging.TopicDriverLocation, payload); err != nil {
					appLog.Errorw("publish location", zap.Error(err))
				}
			}

		case contracts.DriverCmdTripAccept, contracts.DriverCmdTripDecline:
			var acceptData messaging.DriverTripAcceptData
			if err := json.Unmarshal(driverMsg.Data, &acceptData); err != nil {
				appLog.Warnw("unmarshal accept data", zap.Error(err))
				continue
			}

			responseData := messaging.DriverTripResponseData{
				Driver: messaging.DriverInfo{
					ID:             driverData.Driver.Id,
					Name:           driverData.Driver.Name,
					ProfilePicture: driverData.Driver.ProfilePicture,
					CarPlate:       driverData.Driver.CarPlate,
				},
				TripID:  acceptData.TripID,
				RiderID: acceptData.RiderID,
			}

			data, err := json.Marshal(responseData)
			if err != nil {
				appLog.Warnw("marshal trip response", zap.Error(err))
				continue
			}
			payload, err := json.Marshal(messaging.KafkaMessage{
				Type:    driverMsg.Type,
				OwnerID: userID,
				Data:    data,
			})
			if err != nil {
				appLog.Warnw("marshal trip response payload", zap.Error(err))
				continue
			}
			if err := kafkaClient.PublishMessage(r.Context(), messaging.TopicDriverTripResponse, payload); err != nil {
				appLog.Errorw("publish driver response", zap.Error(err))
			}

		case contracts.DriverCmdTripCancel:
			var cancelData struct {
				TripID  string `json:"trip_id"`
				RiderID string `json:"rider_id"`
			}
			if err := json.Unmarshal(driverMsg.Data, &cancelData); err != nil {
				appLog.Warnw("unmarshal driver cancel", zap.Error(err))
				continue
			}
			cancelEvent := messaging.TripCancelledEvent{
				TripID:   cancelData.TripID,
				RiderID:  cancelData.RiderID,
				DriverID: userID,
			}
			eventData, _ := json.Marshal(cancelEvent)
			kmsg := messaging.KafkaMessage{
				Type:    contracts.TripEventCancelledByDriver,
				OwnerID: cancelData.RiderID,
				Data:    eventData,
			}
			payload, _ := json.Marshal(kmsg)
			if err := kafkaClient.PublishMessage(r.Context(), messaging.TopicTripCancelled, payload); err != nil {
				appLog.Errorw("publish driver cancel", zap.Error(err))
			}
			appLog.Infow("driver cancelled trip", "driver", userID, "trip", cancelData.TripID)

		case contracts.DriverCmdArrived, contracts.DriverCmdTripStart, contracts.DriverCmdTripEnd:
			var statusData struct {
				TripID  string `json:"trip_id"`
				RiderID string `json:"rider_id"`
			}
			if err := json.Unmarshal(driverMsg.Data, &statusData); err != nil {
				appLog.Warnw("unmarshal driver status", zap.Error(err))
				continue
			}

			var kafkaTopic, eventType string
			switch driverMsg.Type {
			case contracts.DriverCmdArrived:
				kafkaTopic = messaging.TopicTripDriverArrived
				eventType = contracts.TripEventDriverArrived
			case contracts.DriverCmdTripStart:
				kafkaTopic = messaging.TopicTripInProgress
				eventType = contracts.TripEventInProgress
			case contracts.DriverCmdTripEnd:
				kafkaTopic = messaging.TopicTripCompleted
				eventType = contracts.TripEventCompleted
			}

			statusEvent := messaging.TripStatusEvent{
				TripID:   statusData.TripID,
				RiderID:  statusData.RiderID,
				DriverID: userID,
			}
			data, err := json.Marshal(statusEvent)
			if err != nil {
				appLog.Warnw("marshal status event", zap.Error(err))
				continue
			}
			kmsg := messaging.KafkaMessage{
				Type:    eventType,
				OwnerID: statusData.RiderID,
				Data:    data,
			}
			payload, err := json.Marshal(kmsg)
			if err != nil {
				appLog.Warnw("marshal status payload", zap.Error(err))
				continue
			}
			if err := kafkaClient.PublishMessage(r.Context(), kafkaTopic, payload); err != nil {
				appLog.Errorw("publish driver status", "type", driverMsg.Type, zap.Error(err))
			}
			appLog.Infow("driver status", "driver", userID, "type", driverMsg.Type, "trip", statusData.TripID)

		default:
			appLog.Warnw("unknown driver message", "type", driverMsg.Type)
		}
	}
}
