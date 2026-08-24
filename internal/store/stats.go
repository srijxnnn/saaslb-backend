package store

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"saaslb-backend/internal/domain"
)

const statsMetaID = "stats"

type SiteStats struct {
	Visitors int64
	Visits   int64
	Since    time.Time
}

type statsDoc struct {
	Visitors int64     `bson:"visitors"`
	Visits   int64     `bson:"visits"`
	Since    time.Time `bson:"since"`
}

func (s *Store) EnsureStats(ctx context.Context) error {
	_, err := s.meta().UpdateOne(ctx,
		bson.M{"_id": statsMetaID},
		bson.M{
			"$set": bson.M{"since": domain.LaunchDate},
			"$setOnInsert": bson.M{
				"visitors": int64(0),
				"visits":   int64(0),
			},
		},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		return err
	}

	counted, err := s.visitorsCol().CountDocuments(ctx, bson.M{})
	if err != nil {
		return err
	}

	var doc statsDoc
	if err := s.meta().FindOne(ctx, bson.M{"_id": statsMetaID}).Decode(&doc); err != nil {
		return err
	}

	visitors := doc.Visitors
	if counted > visitors {
		visitors = counted
		_, err = s.meta().UpdateOne(ctx,
			bson.M{"_id": statsMetaID},
			bson.M{"$set": bson.M{"visitors": visitors, "since": domain.LaunchDate}},
		)
		if err != nil {
			return err
		}
	}

	s.statsMu.Lock()
	s.visitors = visitors
	s.visits = doc.Visits
	s.since = domain.LaunchDate
	s.statsMu.Unlock()
	return nil
}

func (s *Store) RecordVisitor(ctx context.Context, visitorID string, countVisit bool) (SiteStats, error) {
	if countVisit {
		if err := s.persistVisit(ctx); err != nil {
			log.Printf("visit persist %s: %v", visitorID, err)
			stats, recErr := s.recordUnique(ctx, visitorID, false)
			if recErr != nil {
				return stats, recErr
			}
			return stats, err
		}
	}

	return s.recordUnique(ctx, visitorID, countVisit)
}

func (s *Store) recordUnique(ctx context.Context, visitorID string, countVisit bool) (SiteStats, error) {
	res, err := s.visitorsCol().UpdateOne(ctx,
		bson.M{"_id": visitorID},
		bson.M{"$setOnInsert": bson.M{"first_seen": time.Now().UTC()}},
		options.UpdateOne().SetUpsert(true),
	)
	if err != nil {
		stats, _ := s.SiteStats(ctx)
		return stats, err
	}

	newVisitor := res.UpsertedCount > 0
	if newVisitor {
		if err := s.persistVisitor(ctx); err != nil {
			log.Printf("visitor persist %s: %v", visitorID, err)
		}
	}

	s.statsMu.Lock()
	if countVisit {
		s.visits++
	}
	if newVisitor {
		s.visitors++
	}
	stats := s.snapshotLocked()
	s.statsMu.Unlock()
	return stats, nil
}

func (s *Store) SiteStats(context.Context) (SiteStats, error) {
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	return s.snapshotLocked(), nil
}

func (s *Store) snapshotLocked() SiteStats {
	return SiteStats{
		Visitors: s.visitors,
		Visits:   s.visits,
		Since:    s.since,
	}
}

func (s *Store) persistVisitor(ctx context.Context) error {
	_, err := s.meta().UpdateOne(ctx,
		bson.M{"_id": statsMetaID},
		bson.M{
			"$inc": bson.M{"visitors": 1},
			"$set": bson.M{"since": domain.LaunchDate},
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}

func (s *Store) persistVisit(ctx context.Context) error {
	_, err := s.meta().UpdateOne(ctx,
		bson.M{"_id": statsMetaID},
		bson.M{
			"$inc": bson.M{"visits": 1},
			"$set": bson.M{"since": domain.LaunchDate},
		},
		options.UpdateOne().SetUpsert(true),
	)
	return err
}
