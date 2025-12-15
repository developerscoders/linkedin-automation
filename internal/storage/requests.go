package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DB) SaveConnectionRequest(ctx context.Context, request *ConnectionRequest) error {
	request.UpdatedAt = time.Now()
	if request.SentAt.IsZero() {
		request.SentAt = time.Now()
	}

	filter := bson.M{"profile_id": request.ProfileID}
	update := bson.M{
		"$set": request,
		"$setOnInsert": bson.M{
			"sent_at":     request.SentAt,
			"retry_count": 0,
		},
	}

	opts := options.Update().SetUpsert(true)
	_, err := db.requests.UpdateOne(ctx, filter, update, opts)

	if err != nil {
		return fmt.Errorf("failed to save connection request: %w", err)
	}

	return nil
}

func (db *DB) UpdateRequestStatus(ctx context.Context, profileID, status string) error {
	filter := bson.M{"profile_id": profileID}
	update := bson.M{
		"$set": bson.M{
			"status":     status,
			"updated_at": time.Now(),
		},
	}

	if status == "accepted" {
		now := time.Now()
		update["$set"].(bson.M)["accepted_at"] = now
	}

	_, err := db.requests.UpdateOne(ctx, filter, update)
	return err
}

func (db *DB) IncrementRetryCount(ctx context.Context, profileID string) error {
	filter := bson.M{"profile_id": profileID}
	update := bson.M{
		"$inc": bson.M{"retry_count": 1},
		"$set": bson.M{
			"updated_at": time.Now(),
		},
	}

	_, err := db.requests.UpdateOne(ctx, filter, update)
	return err
}

func (db *DB) GetConnectionRequest(ctx context.Context, profileID string) (*ConnectionRequest, error) {
	var request ConnectionRequest
	err := db.requests.FindOne(ctx, bson.M{"profile_id": profileID}).Decode(&request)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil // Return nil if not found, let caller handle
		}
		return nil, fmt.Errorf("failed to get connection request: %w", err)
	}
	return &request, nil
}

// Helper to check status (replaces the simplified one we had)
func (db *DB) GetRequestStatus(ctx context.Context, profileID string) (string, error) {
	req, err := db.GetConnectionRequest(ctx, profileID)
	if err != nil {
		return "", err
	}
	if req == nil {
		return "", nil
	}
	return req.Status, nil
}

func (db *DB) GetRequestsByStatus(ctx context.Context, status string, limit int) ([]ConnectionRequest, error) {
	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "sent_at", Value: -1}})

	cursor, err := db.requests.Find(ctx, bson.M{"status": status}, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to get requests by status: %w", err)
	}
	defer cursor.Close(ctx)

	var requests []ConnectionRequest
	if err := cursor.All(ctx, &requests); err != nil {
		return nil, fmt.Errorf("failed to decode requests: %w", err)
	}

	return requests, nil
}

func (db *DB) GetRequestStats(ctx context.Context) (map[string]interface{}, error) {
	pipeline := []bson.M{
		{
			"$group": bson.M{
				"_id": "$status",
				"count": bson.M{"$sum": 1},
			},
		},
	}

	cursor, err := db.requests.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	stats := make(map[string]interface{})

	for cursor.Next(ctx) {
		var result bson.M
		if err := cursor.Decode(&result); err != nil {
			return nil, err
		}
		status := result["_id"].(string)
		count := result["count"]
		stats[status] = count
	}

	// Get total count
	total, err := db.requests.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	stats["total"] = total

	// Calculate acceptance rate
	if accepted, ok := stats["accepted"].(int32); ok {
		if sent, ok := stats["sent"].(int32); ok {
			if sent > 0 {
				rate := float64(accepted) / float64(sent) * 100
				stats["acceptance_rate"] = fmt.Sprintf("%.2f%%", rate)
			}
		}
	}

	return stats, nil
}
