package store

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

func (s *Store) ClaimWebhook(ctx context.Context, id, eventType string) (bool, error) {
	_, err := s.webhooks().InsertOne(ctx, bson.M{
		"_id":         id,
		"event_type":  eventType,
		"received_at": time.Now().UTC(),
	})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func IsNoRows(err error) bool {
	return errors.Is(err, mongo.ErrNoDocuments) || errors.Is(err, ErrNotFound)
}
