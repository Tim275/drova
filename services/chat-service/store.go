package main

import (
	"context"
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
	Content    string             `json:"content" bson:"content"`
	SentAt     time.Time          `json:"sent_at" bson:"sent_at"`
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
	coll.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys: bson.D{{Key: "trip_id", Value: 1}, {Key: "sent_at", Value: 1}},
	})

	return &MessageStore{client: client, coll: coll}, nil
}

func (s *MessageStore) Save(ctx context.Context, tripID, senderID, senderRole, content string) (*Message, error) {
	msg := &Message{
		ID:         primitive.NewObjectID(),
		TripID:     tripID,
		SenderID:   senderID,
		SenderRole: senderRole,
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

func (s *MessageStore) Close() {
	s.client.Disconnect(context.Background())
}
