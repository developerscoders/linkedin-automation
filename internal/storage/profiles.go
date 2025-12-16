package storage

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func (db *DB) SaveProfile(ctx context.Context, profile *Profile) error {
	profile.UpdatedAt = time.Now()
	if profile.DiscoveredAt.IsZero() {
		profile.DiscoveredAt = time.Now()
	}

	filter := bson.M{"linkedin_id": profile.LinkedInID}
	update := bson.M{
		"$set": bson.M{
			"name":       profile.Name,
			"url":        profile.URL,
			"title":      profile.Title,
			"company":    profile.Company,
			"location":   profile.Location,
			"photo_url":  profile.PhotoURL,
			"headline":   profile.Headline,
			"about":      profile.About,
			"updated_at": profile.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"discovered_at": profile.DiscoveredAt,
		},
		"$addToSet": bson.M{
			"tags": bson.M{"$each": profile.Tags},
		},
	}

	opts := options.Update().SetUpsert(true)
	result, err := db.profiles.UpdateOne(ctx, filter, update, opts)

	if err != nil {
		return fmt.Errorf("failed to save profile: %w", err)
	}

	if result.UpsertedID != nil {
		if oid, ok := result.UpsertedID.(primitive.ObjectID); ok {
			profile.ID = oid
		}
	}

	return nil
}

func (db *DB) GetUnprocessedProfiles(ctx context.Context, limit int) ([]Profile, error) {

	pipeline := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "connection_requests",
				"localField":   "linkedin_id",
				"foreignField": "profile_id",
				"as":           "requests",
			},
		},
		{
			"$match": bson.M{
				"requests": bson.M{"$size": 0}, // No connection requests sent
			},
		},
		{
			"$sort": bson.M{"discovered_at": 1},
		},
		{
			"$limit": limit,
		},
	}

	cursor, err := db.profiles.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get unprocessed profiles: %w", err)
	}
	defer cursor.Close(ctx)

	var profiles []Profile
	if err := cursor.All(ctx, &profiles); err != nil {
		return nil, fmt.Errorf("failed to decode profiles: %w", err)
	}

	return profiles, nil
}

func (db *DB) GetProfileByLinkedInID(ctx context.Context, linkedInID string) (*Profile, error) {
	var profile Profile
	err := db.profiles.FindOne(ctx, bson.M{"linkedin_id": linkedInID}).Decode(&profile)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, fmt.Errorf("profile not found: %s", linkedInID)
		}
		return nil, fmt.Errorf("failed to get profile: %w", err)
	}
	return &profile, nil
}

func (db *DB) GetProfilesForConnection(ctx context.Context, limit int) ([]Profile, error) {
	// Get profiles that:
	// 1. Have no connection request OR
	// 2. Have a failed request that can be retried (retry_count < 3)

	pipeline := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "connection_requests",
				"localField":   "linkedin_id",
				"foreignField": "profile_id",
				"as":           "requests",
			},
		},
		{
			"$match": bson.M{
				"$or": []bson.M{
					{"requests": bson.M{"$size": 0}},
					{
						"requests": bson.M{
							"$elemMatch": bson.M{
								"status":      "failed",
								"retry_count": bson.M{"$lt": 3},
							},
						},
					},
				},
			},
		},
		{
			"$sort": bson.M{"discovered_at": 1},
		},
		{
			"$limit": limit,
		},
	}

	cursor, err := db.profiles.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("failed to get profiles for connection: %w", err)
	}
	defer cursor.Close(ctx)

	var profiles []Profile
	if err := cursor.All(ctx, &profiles); err != nil {
		return nil, fmt.Errorf("failed to decode profiles: %w", err)
	}

	return profiles, nil
}

func (db *DB) SearchProfiles(ctx context.Context, query string, limit int) ([]Profile, error) {
	filter := bson.M{
		"$text": bson.M{"$search": query},
	}

	opts := options.Find().
		SetLimit(int64(limit)).
		SetSort(bson.D{{Key: "score", Value: bson.M{"$meta": "textScore"}}}).
		SetProjection(bson.M{"score": bson.M{"$meta": "textScore"}})

	cursor, err := db.profiles.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to search profiles: %w", err)
	}
	defer cursor.Close(ctx)

	var profiles []Profile
	if err := cursor.All(ctx, &profiles); err != nil {
		return nil, fmt.Errorf("failed to decode profiles: %w", err)
	}

	return profiles, nil
}

func (db *DB) GetProfileStats(ctx context.Context) (map[string]interface{}, error) {
	totalProfiles, err := db.profiles.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "connection_requests",
				"localField":   "linkedin_id",
				"foreignField": "profile_id",
				"as":           "requests",
			},
		},
		{
			"$group": bson.M{
				"_id": nil,
				"contacted": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{
							bson.M{"$gt": []interface{}{bson.M{"$size": "$requests"}, 0}},
							1,
							0,
						},
					},
				},
				"uncontacted": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{
							bson.M{"$eq": []interface{}{bson.M{"$size": "$requests"}, 0}},
							1,
							0,
						},
					},
				},
			},
		},
	}

	cursor, err := db.profiles.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result []bson.M
	if err := cursor.All(ctx, &result); err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"total_profiles": totalProfiles,
	}

	if len(result) > 0 {
		stats["contacted"] = result[0]["contacted"]
		stats["uncontacted"] = result[0]["uncontacted"]
	}

	return stats, nil
}
