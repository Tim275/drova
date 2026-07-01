package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"time"

	"drova/services/api-gateway/grpc_clients"
	"drova/shared/contracts"
	"drova/shared/logger"
	"drova/shared/messaging"
	pb "drova/shared/proto/driver"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var numericIDRe = regexp.MustCompile(`^[0-9]+$`)

const (
	wsPingInterval = 25 * time.Second
	wsPongWait     = 35 * time.Second
)

func sendWSError(conn *websocket.Conn, mu *sync.Mutex, message string) {
	msg, err := json.Marshal(contracts.WSMessage{
		Type: contracts.WSEventError,
		Data: map[string]string{"message": message},
	})
	if err != nil {
		return
	}
	mu.Lock()
	conn.WriteMessage(websocket.TextMessage, msg) //nolint:errcheck
	mu.Unlock()
}

var riderConnManager = messaging.NewConnectionManager(nil)
var driverConnManager = messaging.NewConnectionManager(nil)

// subscribeUserWS relays Redis Pub/Sub messages to the WebSocket connection.
// mu must be the same mutex used by the ping goroutine — Gorilla forbids concurrent writes.
func subscribeUserWS(ctx context.Context, channel string, conn *websocket.Conn, mu *sync.Mutex) {
	if gatewayRdb == nil {
		return
	}
	sub := gatewayRdb.Subscribe(ctx, channel)
	defer sub.Close()
	ch := sub.Channel()
	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			mu.Lock()
			err := conn.WriteMessage(websocket.TextMessage, []byte(msg.Payload))
			mu.Unlock()
			if err != nil {
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func handleRidersWebSocket(w http.ResponseWriter, r *http.Request) {
	claims, err := resolveWSAuth(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if claimedID := r.URL.Query().Get("userID"); claimedID == "" || claimedID != fmt.Sprintf("%d", claims.UserID) {
		http.Error(w, "userID mismatch", http.StatusUnauthorized)
		return
	}
	userID := fmt.Sprintf("%d", claims.UserID)
	if !numericIDRe.MatchString(userID) {
		http.Error(w, "invalid userID", http.StatusInternalServerError)
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

	var wmu sync.Mutex
	go func() {
		subscribeUserWS(r.Context(), "ws:rider:"+userID, conn, &wmu)
		conn.Close()
	}()

	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				wmu.Lock()
				err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
				wmu.Unlock()
				if err != nil {
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
			data, err := json.Marshal(cancelEvent)
			if err != nil {
				appLog.Warnw("marshal cancel event", zap.Error(err))
				continue
			}
			ownerID := cancelData.DriverID
			if ownerID == "" {
				ownerID = userID
			}
			payload, err := json.Marshal(messaging.KafkaMessage{
				Type:    contracts.TripEventCancelled,
				OwnerID: ownerID,
				Data:    data,
			})
			if err != nil {
				appLog.Warnw("marshal cancel payload", zap.Error(err))
				continue
			}
			if err := kafkaClient.PublishMessage(r.Context(), messaging.TopicTripCancelled, payload); err != nil {
				appLog.Errorw("publish trip cancel", zap.Error(err))
				sendWSError(conn, &wmu, "Cancel failed, please retry")
			}
			appLog.Infow("trip cancelled", "rider", logger.Clean(userID), "trip", logger.Clean(cancelData.TripID))
		}
	}
}

func handleDriversWebSocket(w http.ResponseWriter, r *http.Request) {
	claims, err := resolveWSAuth(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	if claimedID := r.URL.Query().Get("userID"); claimedID == "" || claimedID != fmt.Sprintf("%d", claims.UserID) {
		http.Error(w, "userID mismatch", http.StatusUnauthorized)
		return
	}
	userID := fmt.Sprintf("%d", claims.UserID)
	if !numericIDRe.MatchString(userID) {
		http.Error(w, "invalid userID", http.StatusInternalServerError)
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

	var wmu sync.Mutex
	go func() {
		ticker := time.NewTicker(wsPingInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				wmu.Lock()
				err := conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(10*time.Second))
				wmu.Unlock()
				if err != nil {
					return
				}
			case <-r.Context().Done():
				return
			}
		}
	}()

	driverName := r.URL.Query().Get("name")

	// Parse real GPS from browser — sent by the frontend before going online.
	var driverLat, driverLng float64
	if v := r.URL.Query().Get("lat"); v != "" {
		if _, err := fmt.Sscanf(v, "%f", &driverLat); err != nil {
			driverLat = 0
		}
	}
	if v := r.URL.Query().Get("lng"); v != "" {
		if _, err := fmt.Sscanf(v, "%f", &driverLng); err != nil {
			driverLng = 0
		}
	}

	ctx := r.Context()

	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cleanupCancel()
		_, _ = grpc_clients.DriverClient.UnregisterDriver(cleanupCtx, &pb.RegisterDriverRequest{
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
			Lat:         driverLat,
			Lng:         driverLng,
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

	var locationStream pb.DriverService_StreamLocationClient
	_, lsErr := grpc_clients.DriverBreaker.Execute(func() (interface{}, error) {
		stream, err := grpc_clients.DriverClient.StreamLocation(ctx)
		if err != nil {
			return nil, err
		}
		locationStream = stream
		return nil, nil
	})
	if lsErr != nil {
		appLog.Warnw("location stream open failed", "driver", claims.UserID, zap.Error(lsErr))
	} else {
		appLog.Infow("location stream opened", "driver", claims.UserID)
	}
	defer func() {
		if locationStream != nil {
			_ = locationStream.CloseSend()
			appLog.Infow("location stream closed", "driver", claims.UserID)
		}
	}()

	driverConnManager.Add(userID, conn)
	defer driverConnManager.Remove(userID)

	var lastDriverLat, lastDriverLng float64
	if driverData.Driver != nil && driverData.Driver.Location != nil {
		lastDriverLat = driverData.Driver.Location.Latitude
		lastDriverLng = driverData.Driver.Location.Longitude
	}

	type registerPayload struct {
		Id             string       `json:"id"`
		Name           string       `json:"name"`
		ProfilePicture string       `json:"profilePicture"`
		CarPlate       string       `json:"carPlate"`
		Geohash        string       `json:"geohash"`
		PackageSlug    string       `json:"packageSlug"`
		Location       *pb.Location `json:"location"`
		BusyWithTripId string       `json:"busyWithTripId,omitempty"`
	}
	regPayload := registerPayload{
		Id:             driverData.Driver.GetId(),
		Name:           driverData.Driver.GetName(),
		ProfilePicture: driverData.Driver.GetProfilePicture(),
		CarPlate:       driverData.Driver.GetCarPlate(),
		Geohash:        driverData.Driver.GetGeohash(),
		PackageSlug:    driverData.Driver.GetPackageSlug(),
		Location:       driverData.Driver.GetLocation(),
		BusyWithTripId: driverData.GetBusyWithTripId(),
	}
	if err := driverConnManager.SendMessage(userID, contracts.WSMessage{
		Type: contracts.DriverCmdRegister,
		Data: regPayload,
	}); err != nil {
		appLog.Errorw("send register message", zap.Error(err))
		return
	}

	go func() {
		subscribeUserWS(r.Context(), "ws:driver:"+userID, conn, &wmu)
		// If the subscriber exits for any reason (e.g. write error on a half-open connection),
		// force-close the WS so the browser receives onclose and auto-reconnects with a fresh subscription.
		conn.Close()
	}()

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
			lastDriverLat = locData.Lat
			lastDriverLng = locData.Lng
			appLog.Debugw("location update", "driver", logger.Clean(userID), "lat", locData.Lat, "lng", locData.Lng, "rider", logger.Clean(locData.RiderID))
			if locationStream != nil {
				if err := locationStream.Send(&pb.LocationUpdate{
					DriverId:  userID,
					Latitude:  locData.Lat,
					Longitude: locData.Lng,
				}); err != nil {
					appLog.Warnw("location stream send", zap.Error(err))
					locationStream = nil
				}
			}
			if locData.RiderID != "" {
				if gatewayRdb != nil {
					wsMsg, merr := json.Marshal(contracts.WSMessage{
						Type: contracts.DriverCmdLocation,
						Data: locData,
					})
					if merr != nil {
						appLog.Warnw("marshal location msg", zap.Error(merr))
					} else if err := gatewayRdb.Publish(r.Context(), "ws:rider:"+locData.RiderID, string(wsMsg)).Err(); err != nil {
						appLog.Warnw("redis publish location", zap.Error(err))
					}
				} else {
					if err := riderConnManager.SendMessage(locData.RiderID, contracts.WSMessage{
						Type: contracts.DriverCmdLocation,
						Data: locData,
					}); err != nil {
						data, merr1 := json.Marshal(locData)
						if merr1 != nil {
							break
						}
						payload, merr2 := json.Marshal(messaging.KafkaMessage{
							Type:    contracts.DriverCmdLocation,
							OwnerID: locData.RiderID,
							Data:    data,
						})
						if merr2 != nil {
							break
						}
						if err := kafkaClient.PublishMessage(r.Context(), messaging.TopicDriverLocation, payload); err != nil {
							appLog.Errorw("publish location", zap.Error(err))
						}
					}
				}
			}

		case contracts.DriverCmdOffline:
			offCtx, offCancel := context.WithTimeout(r.Context(), 3*time.Second)
			_, err := grpc_clients.DriverClient.GoOffline(offCtx, &pb.RegisterDriverRequest{DriverID: userID})
			offCancel()
			if err != nil {
				appLog.Warnw("driver go offline", "driver", logger.Clean(userID), zap.Error(err))
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
					Lat:            lastDriverLat,
					Lng:            lastDriverLng,
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
				sendWSError(conn, &wmu, "Failed to submit response, please retry")
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
			eventData, err := json.Marshal(cancelEvent)
			if err != nil {
				appLog.Warnw("marshal driver cancel event", zap.Error(err))
				continue
			}
			payload, err := json.Marshal(messaging.KafkaMessage{
				Type:    contracts.TripEventCancelledByDriver,
				OwnerID: cancelData.RiderID,
				Data:    eventData,
			})
			if err != nil {
				appLog.Warnw("marshal driver cancel payload", zap.Error(err))
				continue
			}
			if err := kafkaClient.PublishMessage(r.Context(), messaging.TopicTripCancelled, payload); err != nil {
				appLog.Errorw("publish driver cancel", zap.Error(err))
				sendWSError(conn, &wmu, "Cancel failed, please retry")
			}
			appLog.Infow("driver cancelled trip", "driver", logger.Clean(userID), "trip", logger.Clean(cancelData.TripID))

		case contracts.DriverCmdArrived, contracts.DriverCmdTripStart, contracts.DriverCmdTripEnd:
			var statusData struct {
				TripID  string  `json:"trip_id"`
				RiderID string  `json:"rider_id"`
				Lat     float64 `json:"lat"`
				Lng     float64 `json:"lng"`
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
			if driverMsg.Type == contracts.DriverCmdTripEnd && statusData.Lat != 0 {
				statusEvent.ActualLat = statusData.Lat
				statusEvent.ActualLng = statusData.Lng
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
				appLog.Errorw("publish driver status", "type", logger.Clean(driverMsg.Type), zap.Error(err))
				sendWSError(conn, &wmu, "Status update failed, please retry")
			}
			appLog.Infow("driver status", "driver", logger.Clean(userID), "type", logger.Clean(driverMsg.Type), "trip", logger.Clean(statusData.TripID))

		case "ping":
			// client keepalive — ignore
		default:
			appLog.Warnw("unknown driver message", "type", logger.Clean(driverMsg.Type))
		}
	}
}
