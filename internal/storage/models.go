package storage

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Profile struct {
	ID           primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	LinkedInID   string                 `bson:"linkedin_id" json:"linkedin_id"`
	Name         string                 `bson:"name" json:"name"`
	URL          string                 `bson:"url" json:"url"`
	Title        string                 `bson:"title,omitempty" json:"title,omitempty"`
	Company      string                 `bson:"company,omitempty" json:"company,omitempty"`
	Location     string                 `bson:"location,omitempty" json:"location,omitempty"`
	PhotoURL     string                 `bson:"photo_url,omitempty" json:"photo_url,omitempty"`
	Headline     string                 `bson:"headline,omitempty" json:"headline,omitempty"`
	About        string                 `bson:"about,omitempty" json:"about,omitempty"`
	DiscoveredAt time.Time              `bson:"discovered_at" json:"discovered_at"`
	UpdatedAt    time.Time              `bson:"updated_at" json:"updated_at"`
	Tags         []string               `bson:"tags,omitempty" json:"tags,omitempty"`
	Metadata     map[string]interface{} `bson:"metadata,omitempty" json:"metadata,omitempty"`
}

type ConnectionRequest struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProfileID    string             `bson:"profile_id" json:"profile_id"` // Storing LinkedInID string for reference
	ProfileName  string             `bson:"profile_name" json:"profile_name"`
	Note         string             `bson:"note,omitempty" json:"note,omitempty"`
	Status       string             `bson:"status" json:"status"` // sent, accepted, rejected, withdrawn, pending
	SentAt       time.Time          `bson:"sent_at" json:"sent_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
	AcceptedAt   *time.Time         `bson:"accepted_at,omitempty" json:"accepted_at,omitempty"`
	ErrorMessage string             `bson:"error_message,omitempty" json:"error_message,omitempty"`
	RetryCount   int                `bson:"retry_count" json:"retry_count"`
}

type Message struct {
	ID               primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ProfileID        string             `bson:"profile_id" json:"profile_id"`
	ProfileName      string             `bson:"profile_name" json:"profile_name"`
	Content          string             `bson:"content" json:"content"`
	TemplateName     string             `bson:"template_name,omitempty" json:"template_name,omitempty"`
	SentAt           time.Time          `bson:"sent_at" json:"sent_at"`
	Status           string             `bson:"status" json:"status"` // sent, failed, delivered, read
	ErrorMessage     string             `bson:"error_message,omitempty" json:"error_message,omitempty"`
	ResponseReceived bool               `bson:"response_received" json:"response_received"`
}

type ActivityLog struct {
	ID        primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	Action    string                 `bson:"action" json:"action"` // search, connect, message, login, etc.
	ProfileID string                 `bson:"profile_id,omitempty" json:"profile_id,omitempty"`
	Details   map[string]interface{} `bson:"details,omitempty" json:"details,omitempty"`
	Success   bool                   `bson:"success" json:"success"`
	Duration  int64                  `bson:"duration_ms,omitempty" json:"duration_ms,omitempty"` // milliseconds
	CreatedAt time.Time              `bson:"created_at" json:"created_at"`
}

type SessionState struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Key       string             `bson:"key" json:"key"` // "cookies", "last_login", etc.
	Value     interface{}        `bson:"value" json:"value"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

type RateLimitTracker struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ActionType  string             `bson:"action_type" json:"action_type"` // connect, message, search
	Date        string             `bson:"date" json:"date"`               // YYYY-MM-DD
	Hour        int                `bson:"hour" json:"hour"`               // 0-23
	Week        string             `bson:"week" json:"week"`               // YYYY-Www
	Count       int                `bson:"count" json:"count"`
	LastUpdated time.Time          `bson:"last_updated" json:"last_updated"`
}
