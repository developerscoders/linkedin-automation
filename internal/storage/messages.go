package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DB) SaveMessage(ctx context.Context, msg Message) error {
	if msg.SentAt.IsZero() {
		msg.SentAt = time.Now()
	}
	
	_, err := db.messages.InsertOne(ctx, msg)
	if err != nil {
		return fmt.Errorf("failed to save message: %w", err)
	}
	return nil
}

func (db *DB) GetMessageHistory(ctx context.Context, profileID string) ([]Message, error) {
	opts := options.Find().SetSort(bson.D{{Key: "sent_at", Value: 1}})
	
	cursor, err := db.messages.Find(ctx, bson.M{"profile_id": profileID}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get message history: %w", err)
	}
	defer cursor.Close(ctx)

	var messages []Message
	if err := cursor.All(ctx, &messages); err != nil {
		return nil, fmt.Errorf("failed to decode messages: %w", err)
	}
	return messages, nil
}
