package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"drova/services/api-gateway/grpc_clients"
	"drova/shared/contracts"
	"drova/shared/messaging"
	pb "drova/shared/proto/driver"
)

var connManager = messaging.NewConnectionManager()

func handleRidersWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := connManager.Upgrade(w, r)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		log.Println("No user ID provided")
		return
	}

	connManager.Add(userID, conn)
	defer connManager.Remove(userID)

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			break
		}
	}
}

func handleDriversWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := connManager.Upgrade(w, r)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}
	defer conn.Close()

	userID := r.URL.Query().Get("userID")
	if userID == "" {
		log.Println("No user ID provided")
		return
	}

	packageSlug := r.URL.Query().Get("packageSlug")
	if packageSlug == "" {
		log.Println("No package slug provided")
		return
	}

	ctx := r.Context()

	driverService, err := grpc_clients.NewDriverServiceClient()
	if err != nil {
		log.Printf("Failed to connect to driver service: %v", err)
		return
	}

	defer func() {
		driverService.Client.UnregisterDriver(ctx, &pb.RegisterDriverRequest{
			DriverID:    userID,
			PackageSlug: packageSlug,
		})
		driverService.Close()
	}()

	driverData, err := driverService.Client.RegisterDriver(ctx, &pb.RegisterDriverRequest{
		DriverID:    userID,
		PackageSlug: packageSlug,
	})
	if err != nil {
		log.Printf("Error registering driver: %v", err)
		return
	}

	locationStream, err := driverService.Client.StreamLocation(ctx)
	if err != nil {
		log.Printf("Failed to open location stream for driver %s: %v", userID, err)
		locationStream = nil
	} else {
		log.Printf("gRPC location stream opened for driver %s", userID)
	}
	defer func() {
		if locationStream != nil {
			locationStream.CloseSend()
			log.Printf("gRPC location stream closed for driver %s", userID)
		}
	}()

	connManager.Add(userID, conn)
	defer connManager.Remove(userID)

	if err := connManager.SendMessage(userID, contracts.WSMessage{
		Type: contracts.DriverCmdRegister,
		Data: driverData.Driver,
	}); err != nil {
		log.Printf("Error sending register message: %v", err)
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
			log.Printf("Error unmarshaling driver message: %v", err)
			continue
		}

		switch driverMsg.Type {
		case contracts.DriverCmdLocation:
			var locData messaging.DriverLocationData
			if err := json.Unmarshal(driverMsg.Data, &locData); err != nil {
				log.Printf("Error unmarshaling location: %v", err)
				continue
			}
			log.Printf("Location update from driver %s: lat=%.4f lng=%.4f riderID=%s", userID, locData.Lat, locData.Lng, locData.RiderID)
			if locationStream != nil {
				if err := locationStream.Send(&pb.LocationUpdate{
					DriverId:  userID,
					Latitude:  locData.Lat,
					Longitude: locData.Lng,
				}); err != nil {
					log.Printf("Error sending location to driver-service: %v", err)
				}
			}
			if locData.RiderID != "" {
				data, _ := json.Marshal(locData)
				payload, _ := json.Marshal(messaging.KafkaMessage{
					Type:    contracts.DriverCmdLocation,
					OwnerID: locData.RiderID,
					Data:    data,
				})
				if err := kafkaClient.PublishMessage(context.Background(), messaging.TopicDriverLocation, payload); err != nil {
					log.Printf("Error publishing location: %v", err)
				}
			}
		case contracts.DriverCmdTripAccept, contracts.DriverCmdTripDecline:
			var acceptData messaging.DriverTripAcceptData
			if err := json.Unmarshal(driverMsg.Data, &acceptData); err != nil {
				log.Printf("Error unmarshaling accept data: %v", err)
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

			data, _ := json.Marshal(responseData)
			payload, _ := json.Marshal(messaging.KafkaMessage{
				Type:    driverMsg.Type,
				OwnerID: userID,
				Data:    data,
			})

			if err := kafkaClient.PublishMessage(context.Background(), messaging.TopicDriverTripResponse, payload); err != nil {
				log.Printf("Error publishing driver response: %v", err)
			}
		default:
			log.Printf("Unknown driver message: %s", driverMsg.Type)
		}
	}
}
