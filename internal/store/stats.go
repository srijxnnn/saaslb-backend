package store

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	OnlineWindow       = 75 * time.Second
	presenceTTLSeconds = 1800
	statsMetaID        = "stats"
)

type SiteStats struct {
	Online int64
	Visits int64
	Since  time.Time
}

type statsDoc struct {
	Visits int64     `bson:"visits"`
	Since  time.Time `bson:"since"`
}

func (s *Store) EnsureStats(ctx context.Context) error {
	_, err := s.meta().UpdateOne(ctx,
		bson.M{"_id": statsMetaID},
		bson.M{"$setOnInsert": bson.M{
			"visits": int64(0),
			"since":  time.Now().UTC(),
		}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return err
	}

	var doc statsDoc
	if err := s.meta().FindOne(ctx, bson.M{"_id": statsMetaID}).Decode(&doc); err != nil {
		return err
	}

	s.liveMu.Lock()
	s.visits = doc.Visits
	s.since = doc.Since
	s.liveMu.Unlock()
	return nil
}

func (s *Store) TouchPresence(_ context.Context, visitorID string, countVisit bool) (SiteStats, error) {
	stats, claimed := s.touchLive(visitorID, countVisit)
	if claimed {
		go s.persistVisit(visitorID)
	}
	return stats, nil
}

func (s *Store) SiteStats(context.Context) (SiteStats, error) {
	return s.snapshotLive(), nil
}

func (s *Store) touchLive(visitorID string, countVisit bool) (SiteStats, bool) {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()

	s.live[visitorID] = time.Now().UTC()

	claimed := false
	if countVisit {
		s.visits++
		claimed = true
	}

	return s.snapshotLocked(), claimed
}

func (s *Store) snapshotLive() SiteStats {
	s.liveMu.Lock()
	defer s.liveMu.Unlock()
	return s.snapshotLocked()
}

func (s *Store) snapshotLocked() SiteStats {
	now := time.Now().UTC()
	cutoff := now.Add(-OnlineWindow)
	for id, seen := range s.live {
		if seen.Before(cutoff) {
			delete(s.live, id)
		}
	}

	return SiteStats{
		Online: int64(len(s.live)),
		Visits: s.visits,
		Since:  s.since,
	}
}

func (s *Store) persistVisit(visitorID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()

	now := time.Now().UTC()
	if _, err := s.presence().UpdateOne(ctx,
		bson.M{"_id": visitorID},
		bson.M{"$set": bson.M{"last_seen": now}},
		options.UpdateOne().SetUpsert(true),
	); err != nil {
		log.Printf("presence persist %s: %v", visitorID, err)
	}

	if _, err := s.meta().UpdateOne(ctx,
		bson.M{"_id": statsMetaID},
		bson.M{"$inc": bson.M{"visits": 1}},
	); err != nil {
		log.Printf("visit count %s: %v", visitorID, err)
		s.rollbackVisit()
	}
}

func (s *Store) rollbackVisit() {
	s.liveMu.Lock()
	if s.visits > 0 {
		s.visits--
	}
	s.liveMu.Unlock()
}
