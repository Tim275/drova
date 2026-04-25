package main

import (
	"context"
	"encoding/json"
	"log"

	"drova/shared/contracts"
	"drova/shared/messaging"
)

func startNotificationConsumers(ctx context.Context) {
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicDriverTripRequest, "api-gateway-driver-notify", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicTripDriverAssigned, "api-gateway-rider-notify", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicTripNoDriversFound, "api-gateway-no-drivers", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicPaymentSessionCreated, "api-gateway-payment-session", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicDriverLocation, "api-gateway-location-notify", handleNotification)
}

func handleNotification(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return err
	}

	log.Printf("Notification received: type=%s owner=%s → dispatching to WS", msg.Type, msg.OwnerID)

	if err := connManager.SendMessage(msg.OwnerID, contracts.WSMessage{
		Type: msg.Type,
		Data: json.RawMessage(msg.Data),
	}); err != nil {
		if err != messaging.ErrConnectionNotFound {
			log.Printf("Error sending notification to %s: %v", msg.OwnerID, err)
		} else {
			log.Printf("No active WS for %s — dropping notification", msg.OwnerID)
		}
	} else {
		log.Printf("WS dispatched: type=%s → %s", msg.Type, msg.OwnerID)
	}
	return nil
}
