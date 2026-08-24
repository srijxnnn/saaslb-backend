package store

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"saaslb-backend/internal/domain"
)

func (s *Store) recordClickEvent(ctx context.Context, productID string) error {
	_, err := s.clicks().InsertOne(ctx, clickDoc{
		ID:        NewID("clk_"),
		ProductID: productID,
		CreatedAt: time.Now().UTC(),
	})
	return err
}

func (s *Store) clicksInWindow(ctx context.Context, since time.Time) (map[string]int, error) {
	cursor, err := s.clicks().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: bson.M{"created_at": bson.M{"$gte": since}}}},
		{{Key: "$group", Value: bson.M{
			"_id":   "$product_id",
			"count": bson.M{"$sum": 1},
		}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		ID    string `bson:"_id"`
		Count int    `bson:"count"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}

	counts := make(map[string]int, len(rows))
	for _, row := range rows {
		counts[row.ID] = row.Count
	}
	return counts, nil
}

func (s *Store) clicksInWindowFor(ctx context.Context, productID string) (int, error) {
	n, err := s.clicks().CountDocuments(ctx, bson.M{
		"product_id": productID,
		"created_at": bson.M{"$gte": time.Now().UTC().Add(-domain.ClickWindow)},
	})
	return int(n), err
}

func (s *Store) applyClickWindow(ctx context.Context, products []domain.Product) {
	if len(products) == 0 {
		return
	}

	counts, err := s.clicksInWindow(ctx, time.Now().UTC().Add(-domain.ClickWindow))
	if err != nil {
		log.Printf("click window: %v", err)
		return
	}

	for i := range products {
		products[i].ClicksLastHour = counts[products[i].ID]
	}
}

func (s *Store) applyClickWindowOne(ctx context.Context, product *domain.Product) {
	n, err := s.clicksInWindowFor(ctx, product.ID)
	if err != nil {
		log.Printf("click window %s: %v", product.ID, err)
		return
	}
	product.ClicksLastHour = n
}
