package main

import (
	"context"
	"encoding/json"

	"drova/shared/contracts"
	"drova/shared/messaging"
)

func startNotificationConsumers(ctx context.Context) {
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicDriverTripRequest, "api-gateway-driver-notify", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicTripDriverAssigned, "api-gateway-rider-notify", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicTripNoDriversFound, "api-gateway-no-drivers", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicPaymentSessionCreated, "api-gateway-payment-session", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicDriverLocation, "api-gateway-location-notify", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicTripCancelled, "api-gateway-cancelled", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicTripDriverArrived, "api-gateway-arrived", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicTripInProgress, "api-gateway-in-progress", handleNotification)
	go kafkaClient.ConsumeMessages(ctx, messaging.TopicTripCompleted, "api-gateway-completed", handleNotification)
}

func handleNotification(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return err
	}

	appLog.Infow("notification", "type", msg.Type, "owner", msg.OwnerID)

	cm := riderConnManager
	if msg.Type == messaging.TopicDriverTripRequest || msg.Type == messaging.TopicTripCancelled {
		cm = driverConnManager
	}

	if err := cm.SendMessage(msg.OwnerID, contracts.WSMessage{
		Type: msg.Type,
		Data: json.RawMessage(msg.Data),
	}); err != nil {
		if err != messaging.ErrConnectionNotFound {
			appLog.Warnw("send notification failed", "owner", msg.OwnerID, "error", err)
		}
	} else {
		appLog.Infow("ws dispatched", "type", msg.Type, "owner", msg.OwnerID)
	}
	return nil
}
