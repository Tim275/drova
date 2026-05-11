package main

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Message struct {
	ID         primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	TripID     string             `json:"trip_id" bson:"trip_id"`
	SenderID   string             `json:"sender_id" bson:"sender_id"`
	SenderRole string             `json:"sender_role" bson:"sender_role"`
	SenderName string             `json:"sender_name" bson:"sender_name"`
	Content    string             `json:"content" bson:"content"`
	SentAt     time.Time          `json:"sent_at" bson:"sent_at"`
	ReadAt     *time.Time         `json:"read_at,omitempty" bson:"read_at,omitempty"`
	DeletedAt  *time.Time         `json:"deleted_at,omitempty" bson:"deleted_at,omitempty"`
}

type MessageStore struct {
	client *mongo.Client
	coll   *mongo.Collection
}

func NewMessageStore(ctx context.Context, uri string) (*MessageStore, error) {
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err := client.Ping(ctx, nil); err != nil {
		return nil, err
	}

	coll := client.Database("drova").Collection("messages")
	_, _ = coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "trip_id", Value: 1}, {Key: "sent_at", Value: 1}},
	})

	return &MessageStore{client: client, coll: coll}, nil
}

func (s *MessageStore) Save(ctx context.Context, tripID, senderID, senderRole, senderName, content string) (*Message, error) {
	msg := &Message{
		ID:         primitive.NewObjectID(),
		TripID:     tripID,
		SenderID:   senderID,
		SenderRole: senderRole,
		SenderName: senderName,
		Content:    content,
		SentAt:     time.Now().UTC(),
	}
	_, err := s.coll.InsertOne(ctx, msg)
	return msg, err
}

func (s *MessageStore) GetMessages(ctx context.Context, tripID string, limit int64) ([]*Message, error) {
	opts := options.Find().
		SetSort(bson.D{{Key: "sent_at", Value: 1}}).
		SetLimit(limit)

	cursor, err := s.coll.Find(ctx, bson.M{"trip_id": tripID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var msgs []*Message
	if err := cursor.All(ctx, &msgs); err != nil {
		return nil, err
	}
	return msgs, nil
}

// MarkAllRead marks all messages in a trip NOT sent by readerID as read.
// Returns the timestamp used so callers can broadcast it.
func (s *MessageStore) MarkAllRead(ctx context.Context, tripID, readerID string) (time.Time, error) {
	if _, err := strconv.ParseInt(readerID, 10, 64); err != nil {
		return time.Time{}, fmt.Errorf("invalid readerID")
	}
	now := time.Now().UTC()
	_, err := s.coll.UpdateMany(ctx,
		bson.M{
			"trip_id":   tripID,
			"sender_id": bson.M{"$ne": readerID},
			"read_at":   nil,
			"deleted_at": nil,
		},
		bson.M{"$set": bson.M{"read_at": now}},
	)
	return now, err
}

// SoftDelete marks a message as deleted (only by the original sender).
func (s *MessageStore) SoftDelete(ctx context.Context, messageID, senderID string) (*Message, error) {
	oid, err := primitive.ObjectIDFromHex(messageID)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	var msg Message
	err = s.coll.FindOneAndUpdate(ctx,
		bson.M{"_id": oid, "sender_id": senderID, "deleted_at": nil},
		bson.M{"$set": bson.M{"deleted_at": now, "content": ""}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&msg)
	return &msg, err
}

func (s *MessageStore) Close() {
	_ = s.client.Disconnect(context.Background())
}
