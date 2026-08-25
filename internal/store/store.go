package store

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

type Store struct {
	client *mongo.Client
	db     *mongo.Database

	statsMu  sync.Mutex
	visitors int64
	visits   int64
	since    time.Time
}

func Open(ctx context.Context, uri, database string) (*Store, error) {
	if database == "" {
		database = "saaslb"
	}

	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	if err := client.Ping(ctx, nil); err != nil {
		_ = client.Disconnect(ctx)
		return nil, fmt.Errorf("ping: %w", err)
	}

	return &Store{
		client: client,
		db:     client.Database(database),
	}, nil
}

func (s *Store) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = s.client.Disconnect(ctx)
}

func (s *Store) products() *mongo.Collection {
	return s.db.Collection("products")
}

func (s *Store) checkouts() *mongo.Collection {
	return s.db.Collection("checkouts")
}

func (s *Store) webhooks() *mongo.Collection {
	return s.db.Collection("webhook_events")
}

func (s *Store) meta() *mongo.Collection {
	return s.db.Collection("meta")
}

func (s *Store) visitorsCol() *mongo.Collection {
	return s.db.Collection("visitors")
}

func (s *Store) clicks() *mongo.Collection {
	return s.db.Collection("clicks")
}

// Migrate creates unique and sort indexes. MongoDB has no CREATE TABLE, so
// uniqueness that used to live in Postgres constraints lives here instead.
func (s *Store) Migrate(ctx context.Context) error {
	_, err := s.products().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "slug", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "listing_key", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "bid_cents", Value: -1}, {Key: "created_at", Value: 1}},
		},
	})
	if err != nil {
		return fmt.Errorf("products indexes: %w", err)
	}

	// session_id and payment_id are optional until Dodo assigns them.
	// A partial unique index allows many checkouts with no session yet.
	_, err = s.checkouts().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "session_id", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"session_id": bson.M{"$type": "string"}}),
		},
		{
			Keys: bson.D{{Key: "payment_id", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"payment_id": bson.M{"$type": "string"}}),
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "product_id", Value: 1},
				{Key: "paid_at", Value: 1},
			},
		},
		{
			Keys: bson.D{
				{Key: "status", Value: 1},
				{Key: "paid_at", Value: -1},
				{Key: "created_at", Value: -1},
			},
		},
	})
	if err != nil {
		return fmt.Errorf("checkouts indexes: %w", err)
	}

	if err := s.EnsureStats(ctx); err != nil {
		return fmt.Errorf("stats: %w", err)
	}

	if err := dropIndexIfExists(ctx, s.clicks(), "created_at_1"); err != nil {
		return fmt.Errorf("drop clicks ttl: %w", err)
	}
	if err := dropIndexIfExists(ctx, s.clicks(), "product_id_1_visitor_id_1"); err != nil {
		return fmt.Errorf("drop clicks visitor unique: %w", err)
	}

	_, err = s.clicks().Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "created_at", Value: -1}},
		},
		{
			Keys: bson.D{{Key: "product_id", Value: 1}, {Key: "created_at", Value: 1}},
		},
		{
			Keys: bson.D{{Key: "dedup_key", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetPartialFilterExpression(bson.M{"dedup_key": bson.M{"$type": "string"}}),
		},
	})
	if err != nil {
		return fmt.Errorf("clicks indexes: %w", err)
	}

	return nil
}

func dropIndexIfExists(ctx context.Context, coll *mongo.Collection, name string) error {
	err := coll.Indexes().DropOne(ctx, name)
	if err == nil {
		return nil
	}
	var cmd mongo.CommandError
	if errors.As(err, &cmd) && (cmd.Code == 27 || cmd.Name == "IndexNotFound") {
		return nil
	}
	return err
}
