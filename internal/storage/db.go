package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DB struct {
	client   *mongo.Client
	database *mongo.Database

	// Collections
	profiles     *mongo.Collection
	requests     *mongo.Collection
	messages     *mongo.Collection
	activityLog  *mongo.Collection
	sessionState *mongo.Collection
	rateLimits   *mongo.Collection
}

type Config struct {
	URI      string
	Database string
	Timeout  time.Duration
}

func New(cfg *Config) (*DB, error) {
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Timeout)
	defer cancel()

	// Set client options
	clientOptions := options.Client().
		ApplyURI(cfg.URI).
		SetMaxPoolSize(50).
		SetMinPoolSize(10).
		SetMaxConnIdleTime(30 * time.Second)

	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MongoDB: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("failed to ping MongoDB: %w", err)
	}

	database := client.Database(cfg.Database)

	db := &DB{
		client:       client,
		database:     database,
		profiles:     database.Collection("profiles"),
		requests:     database.Collection("connection_requests"),
		messages:     database.Collection("messages"),
		activityLog:  database.Collection("activity_log"),
		sessionState: database.Collection("session_state"),
		rateLimits:   database.Collection("rate_limits"),
	}

	if err := db.createIndexes(ctx); err != nil {
		return nil, fmt.Errorf("failed to create indexes: %w", err)
	}

	return db, nil
}

func (db *DB) createIndexes(ctx context.Context) error {
	// Profile indexes
	profileIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "linkedin_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "url", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "discovered_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "name", Value: "text"}, {Key: "company", Value: "text"}, {Key: "title", Value: "text"}},
		},
		{
			Keys: bson.D{{Key: "tags", Value: 1}},
		},
	}

	_, err := db.profiles.Indexes().CreateMany(ctx, profileIndexes)
	if err != nil {
		return fmt.Errorf("failed to create profile indexes: %w", err)
	}

	// Connection request indexes
	requestIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "profile_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "sent_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}, {Key: "sent_at", Value: -1}},
		},
	}

	_, err = db.requests.Indexes().CreateMany(ctx, requestIndexes)
	if err != nil {
		return fmt.Errorf("failed to create request indexes: %w", err)
	}

	// Message indexes
	messageIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "profile_id", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "sent_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "status", Value: 1}},
		},
	}

	_, err = db.messages.Indexes().CreateMany(ctx, messageIndexes)
	if err != nil {
		return fmt.Errorf("failed to create message indexes: %w", err)
	}

	// Activity log indexes
	activityIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "action", Value: 1}, {Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "profile_id", Value: 1}},
		},
	}

	_, err = db.activityLog.Indexes().CreateMany(ctx, activityIndexes)
	if err != nil {
		return fmt.Errorf("failed to create activity log indexes: %w", err)
	}

	// Session state indexes
	sessionIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "key", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}

	_, err = db.sessionState.Indexes().CreateMany(ctx, sessionIndexes)
	if err != nil {
		return fmt.Errorf("failed to create session state indexes: %w", err)
	}

	// Rate limit indexes
	rateLimitIndexes := []mongo.IndexModel{
		{
			Keys: bson.D{
				{Key: "action_type", Value: 1},
				{Key: "date", Value: 1},
			},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{
				{Key: "action_type", Value: 1},
				{Key: "date", Value: 1},
				{Key: "hour", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "action_type", Value: 1},
				{Key: "week", Value: 1},
			},
		},
	}

	_, err = db.rateLimits.Indexes().CreateMany(ctx, rateLimitIndexes)
	if err != nil {
		return fmt.Errorf("failed to create rate limit indexes: %w", err)
	}

	return nil
}

func (db *DB) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return db.client.Disconnect(ctx)
}

func (db *DB) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return db.client.Ping(ctx, nil)
}
