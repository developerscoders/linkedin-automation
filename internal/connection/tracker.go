package connection

import (
	"context"
	"errors"
	"time"

	"linkedin-automation/internal/storage"
	"go.mongodb.org/mongo-driver/mongo"
)

type Tracker struct {
	storage *storage.DB
}

func NewTracker(db *storage.DB) *Tracker {
	return &Tracker{storage: db}
}

func (t *Tracker) TrackRequest(ctx context.Context, profileID, profileName, note string) error {
	req := &storage.ConnectionRequest{
		ProfileID:   profileID,
		ProfileName: profileName,
		Note:        note,
		Status:      "pending",
		SentAt:      time.Now(),
	}
	return t.storage.SaveConnectionRequest(ctx, req)
}

func (t *Tracker) IsAlreadySent(ctx context.Context, profileID string) (bool, error) {
	status, err := t.storage.GetRequestStatus(ctx, profileID)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return false, nil
		}
		return false, err
	}
	return status != "", nil
}

func (t *Tracker) UpdateStatus(ctx context.Context, profileID, status string) error {
	return t.storage.UpdateRequestStatus(ctx, profileID, status)
}
