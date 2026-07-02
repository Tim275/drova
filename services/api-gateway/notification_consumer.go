package main

import (
	"context"
	"encoding/json"
	"time"

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
	kafkaClient.StartRetryConsumers(ctx, "api-gateway")
}

func handleNotification(ctx context.Context, payload []byte) error {
	var msg messaging.KafkaMessage
	if err := json.Unmarshal(payload, &msg); err != nil {
		return err
	}

	appLog.Infow("notification", "type", msg.Type, "owner", msg.OwnerID)

	// Record chat participants (rider + driver) so the chat-service can enforce a
	// trip-participation check on connect. driver_assigned carries both IDs.
	if msg.Type == messaging.TopicTripDriverAssigned && gatewayRdb != nil {
		var d struct {
			ID     string `json:"id"` // driver
			TripID string `json:"trip_id"`
		}
		if json.Unmarshal(msg.Data, &d) == nil && d.TripID != "" {
			key := "chat:participants:" + d.TripID
			var members []interface{}
			if msg.OwnerID != "" {
				members = append(members, msg.OwnerID) // rider
			}
			if d.ID != "" {
				members = append(members, d.ID) // driver
			}
			if len(members) > 0 {
				if err := gatewayRdb.SAdd(ctx, key, members...).Err(); err != nil {
					appLog.Warnw("chat participants sadd", "trip", d.TripID, "error", err)
				}
				gatewayRdb.Expire(ctx, key, 24*time.Hour)
			}
		}
	}

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
