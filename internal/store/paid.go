package store

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"saaslb-backend/internal/domain"
)

type paidWindows struct {
	Daily         int
	Weekly        int
	Monthly       int
	All           int
	LastPaidAt    time.Time
	LastPaidCents int
}

func (s *Store) paidWindowsByProduct(ctx context.Context, now time.Time, productID string) (map[string]paidWindows, error) {
	match := bson.M{
		"status":     "paid",
		"product_id": bson.M{"$type": "string"},
	}
	if productID != "" {
		match["product_id"] = productID
	}

	dailyFrom := now.Add(-domain.RankWindowDaily)
	weeklyFrom := now.Add(-domain.RankWindowWeekly)
	monthlyFrom := now.Add(-domain.RankWindowMonthly)

	cursor, err := s.checkouts().Aggregate(ctx, mongo.Pipeline{
		{{Key: "$match", Value: match}},
		{{Key: "$addFields", Value: bson.M{
			"paid_at_at": bson.M{"$ifNull": bson.A{"$paid_at", "$created_at"}},
		}}},
		{{Key: "$sort", Value: bson.D{{Key: "paid_at_at", Value: -1}}}},
		{{Key: "$group", Value: bson.M{
			"_id":             "$product_id",
			"last_paid_at":    bson.M{"$first": "$paid_at_at"},
			"last_paid_cents": bson.M{"$first": "$paid_cents"},
			"all":             bson.M{"$sum": "$paid_cents"},
			"daily": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$gte": bson.A{"$paid_at_at", dailyFrom}},
				"$paid_cents",
				0,
			}}},
			"weekly": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$gte": bson.A{"$paid_at_at", weeklyFrom}},
				"$paid_cents",
				0,
			}}},
			"monthly": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$gte": bson.A{"$paid_at_at", monthlyFrom}},
				"$paid_cents",
				0,
			}}},
		}}},
	})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		ID            string    `bson:"_id"`
		All           int       `bson:"all"`
		Daily         int       `bson:"daily"`
		Weekly        int       `bson:"weekly"`
		Monthly       int       `bson:"monthly"`
		LastPaidAt    time.Time `bson:"last_paid_at"`
		LastPaidCents int       `bson:"last_paid_cents"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}

	out := make(map[string]paidWindows, len(rows))
	for _, row := range rows {
		out[row.ID] = paidWindows{
			Daily:         row.Daily,
			Weekly:        row.Weekly,
			Monthly:       row.Monthly,
			All:           row.All,
			LastPaidAt:    row.LastPaidAt,
			LastPaidCents: row.LastPaidCents,
		}
	}
	return out, nil
}

func applyPaidWindow(product *domain.Product, windows paidWindows) {
	product.PaidDailyCents = windows.Daily
	product.PaidWeeklyCents = windows.Weekly
	product.PaidMonthlyCents = windows.Monthly
	product.PaidAllTimeCents = windows.All
	if product.PaidAllTimeCents < product.BidCents {
		product.PaidAllTimeCents = product.BidCents
	}
	if !windows.LastPaidAt.IsZero() {
		paidAt := windows.LastPaidAt
		product.LastPaidAt = &paidAt
		product.LastPaidCents = windows.LastPaidCents
	}
}

func (s *Store) applyPaidWindows(ctx context.Context, products []domain.Product) {
	if len(products) == 0 {
		return
	}

	windows, err := s.paidWindowsByProduct(ctx, time.Now().UTC(), "")
	if err != nil {
		log.Printf("paid windows: %v", err)
		return
	}

	for i := range products {
		applyPaidWindow(&products[i], windows[products[i].ID])
	}
}

func (s *Store) applyPaidWindowsOne(ctx context.Context, product *domain.Product) {
	windows, err := s.paidWindowsByProduct(ctx, time.Now().UTC(), product.ID)
	if err != nil {
		log.Printf("paid windows %s: %v", product.ID, err)
		return
	}
	applyPaidWindow(product, windows[product.ID])
}
