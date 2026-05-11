package main

import (
	"context"
	"encoding/json"

	"drova/shared/contracts"
	"drova/shared/messaging"
)

func startNotificationConsumers(ctx context.Context) {
	kafkaClient.ConsumeMessages(ctx, messaging.TopicDriverTripRequest, "api-gateway-driver-notify", handleNotification)
	kafkaClient.ConsumeMessages(ctx, messaging.TopicTripDriverAssigned, "api-gateway-rider-notify", handleNotification)
	kafkaClient.ConsumeMessages(ctx, messaging.TopicTripNoDriversFound, "api-gateway-no-drivers", handleNotification)
	kafkaClient.ConsumeMessages(ctx, messaging.TopicPaymentSessionCreated, "api-gateway-payment-session", handleNotification)
	kafkaClient.ConsumeMessages(ctx, messaging.TopicDriverLocation, "api-gateway-location-notify", handleNotification)
	kafkaClient.ConsumeMessages(ctx, messaging.TopicTripCancelled, "api-gateway-cancelled", handleNotification)
	kafkaClient.ConsumeMessages(ctx, messaging.TopicTripDriverArrived, "api-gateway-arrived", handleNotification)
	kafkaClient.ConsumeMessages(ctx, messaging.TopicTripInProgress, "api-gateway-in-progress", handleNotification)
	kafkaClient.ConsumeMessages(ctx, messaging.TopicTripCompleted, "api-gateway-completed", handleNotification)
}

func handleNotification(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return err
	}

	appLog.Infow("notification", "type", msg.Type, "owner", msg.OwnerID)

	wsMsg, err := json.Marshal(contracts.WSMessage{
		Type: msg.Type,
		Data: json.RawMessage(msg.Data),
	})
	if err != nil {
		return err
	}

	channel := "ws:rider:" + msg.OwnerID
	if msg.Type == messaging.TopicDriverTripRequest || msg.Type == messaging.TopicTripCancelled {
		channel = "ws:driver:" + msg.OwnerID
	}

	if gatewayRdb != nil {
		if err := gatewayRdb.Publish(ctx, channel, string(wsMsg)).Err(); err != nil {
			appLog.Warnw("redis publish notification", "channel", channel, "error", err)
		} else {
			appLog.Infow("ws dispatched", "type", msg.Type, "owner", msg.OwnerID)
		}
		return nil
	}

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
