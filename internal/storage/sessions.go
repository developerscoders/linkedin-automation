package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DB) SaveSession(ctx context.Context, key string, value interface{}) error {
	filter := bson.M{"key": key}
	update := bson.M{
		"$set": bson.M{
			"value":      value,
			"updated_at": time.Now(),
		},
	}
	opts := options.Update().SetUpsert(true)

	_, err := db.sessionState.UpdateOne(ctx, filter, update, opts)
	if err != nil {
		return fmt.Errorf("failed to save session: %w", err)
	}
	return nil
}

func (db *DB) GetSession(ctx context.Context, key string) (string, error) {
	var session SessionState
	err := db.sessionState.FindOne(ctx, bson.M{"key": key}).Decode(&session)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return "", nil 
		}
		return "", fmt.Errorf("failed to get session: %w", err)
	}
	
	// Assuming value is stored as string for cookies implementation
	if strVal, ok := session.Value.(string); ok {
		return strVal, nil
	}
	
	// If it's not a string, return error or handle accordingly
	return fmt.Sprintf("%v", session.Value), nil
}
