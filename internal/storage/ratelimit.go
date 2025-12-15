package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DB) IncrementRateLimit(ctx context.Context, actionType string) error {
	now := time.Now()
	date := now.Format("2006-01-02")
	hour := now.Hour()
	_, week := now.ISOWeek()
	weekStr := fmt.Sprintf("%d-W%02d", now.Year(), week)

	// Update daily count
	filter := bson.M{
		"action_type": actionType,
		"date":        date,
	}
	update := bson.M{
		"$inc": bson.M{"count": 1},
		"$set": bson.M{
			"last_updated": now,
			"week":         weekStr,
		},
	}
	opts := options.Update().SetUpsert(true)

	if _, err := db.rateLimits.UpdateOne(ctx, filter, update, opts); err != nil {
		return fmt.Errorf("failed to update daily rate limit: %w", err)
	}

	// Update hourly count
	filterHourly := bson.M{
		"action_type": actionType,
		"date":        date,
		"hour":        hour,
	}

	if _, err := db.rateLimits.UpdateOne(ctx, filterHourly, update, opts); err != nil {
		return fmt.Errorf("failed to update hourly rate limit: %w", err)
	}

	return nil
}

func (db *DB) GetRateLimitCount(ctx context.Context, actionType, period string) (int, error) {
	now := time.Now()
	var filter bson.M

	switch period {
	case "daily":
		filter = bson.M{
			"action_type": actionType,
			"date":        now.Format("2006-01-02"),
		}
	case "hourly":
		filter = bson.M{
			"action_type": actionType,
			"date":        now.Format("2006-01-02"),
			"hour":        now.Hour(),
		}
	case "weekly":
		_, week := now.ISOWeek()
		filter = bson.M{
			"action_type": actionType,
			"week":        fmt.Sprintf("%d-W%02d", now.Year(), week),
		}
	default:
		return 0, fmt.Errorf("invalid period: %s", period)
	}

	var tracker RateLimitTracker
	err := db.rateLimits.FindOne(ctx, filter).Decode(&tracker)
	if err != nil {
		if err.Error() == "mongo: no documents in result" {
			return 0, nil
		}
		return 0, fmt.Errorf("failed to get rate limit count: %w", err)
	}

	return tracker.Count, nil
}

func (db *DB) CanPerformAction(ctx context.Context, actionType string, dailyLimit, hourlyLimit, weeklyLimit int) (bool, string, error) {
	// Check hourly limit
	hourlyCount, err := db.GetRateLimitCount(ctx, actionType, "hourly")
	if err != nil {
		return false, "", err
	}
	if hourlyCount >= hourlyLimit {
		return false, "hourly limit reached", nil
	}

	// Check daily limit
	dailyCount, err := db.GetRateLimitCount(ctx, actionType, "daily")
	if err != nil {
		return false, "", err
	}
	if dailyCount >= dailyLimit {
		return false, "daily limit reached", nil
	}

	// Check weekly limit
	weeklyCount, err := db.GetRateLimitCount(ctx, actionType, "weekly")
	if err != nil {
		return false, "", err
	}
	if weeklyCount >= weeklyLimit {
		return false, "weekly limit reached", nil
	}

	return true, "", nil
}
