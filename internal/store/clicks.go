package store

import (
	"context"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"

	"saaslb-backend/internal/domain"
)

type clickWindows struct {
	LastHour int
	Daily    int
	Weekly   int
	Monthly  int
	All      int
}

func (s *Store) recordClickEvent(ctx context.Context, productID, visitorID string) error {
	now := time.Now().UTC()
	_, err := s.clicks().InsertOne(ctx, clickDoc{
		ID:        NewID("clk_"),
		ProductID: productID,
		VisitorID: visitorID,
		DedupKey:  domain.ClickDedupKey(productID, visitorID, now),
		CreatedAt: now,
	})
	return err
}

func (s *Store) clickWindowsByProduct(ctx context.Context, now time.Time, productID string) (map[string]clickWindows, error) {
	match := bson.M{}
	if productID != "" {
		match["product_id"] = productID
	}

	hourFrom := now.Add(-domain.ClickWindow)
	dailyFrom := now.Add(-domain.RankWindowDaily)
	weeklyFrom := now.Add(-domain.RankWindowWeekly)
	monthlyFrom := now.Add(-domain.RankWindowMonthly)

	pipeline := mongo.Pipeline{
		{{Key: "$group", Value: bson.M{
			"_id": "$product_id",
			"all": bson.M{"$sum": 1},
			"hour": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$gte": bson.A{"$created_at", hourFrom}},
				1,
				0,
			}}},
			"daily": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$gte": bson.A{"$created_at", dailyFrom}},
				1,
				0,
			}}},
			"weekly": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$gte": bson.A{"$created_at", weeklyFrom}},
				1,
				0,
			}}},
			"monthly": bson.M{"$sum": bson.M{"$cond": bson.A{
				bson.M{"$gte": bson.A{"$created_at", monthlyFrom}},
				1,
				0,
			}}},
		}}},
	}
	if len(match) > 0 {
		pipeline = append(mongo.Pipeline{{{Key: "$match", Value: match}}}, pipeline...)
	}

	cursor, err := s.clicks().Aggregate(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var rows []struct {
		ID      string `bson:"_id"`
		All     int    `bson:"all"`
		Hour    int    `bson:"hour"`
		Daily   int    `bson:"daily"`
		Weekly  int    `bson:"weekly"`
		Monthly int    `bson:"monthly"`
	}
	if err := cursor.All(ctx, &rows); err != nil {
		return nil, err
	}

	out := make(map[string]clickWindows, len(rows))
	for _, row := range rows {
		out[row.ID] = clickWindows{
			LastHour: row.Hour,
			Daily:    row.Daily,
			Weekly:   row.Weekly,
			Monthly:  row.Monthly,
			All:      row.All,
		}
	}
	return out, nil
}

func applyClickWindow(product *domain.Product, windows clickWindows) {
	product.ClicksLastHour = windows.LastHour
	product.ClicksDaily = windows.Daily
	product.ClicksWeekly = windows.Weekly
	product.ClicksMonthly = windows.Monthly
	product.ClicksAllTime = windows.All
	if product.ClicksAllTime < product.Clicks {
		product.ClicksAllTime = product.Clicks
	}
}

func (s *Store) applyClickWindow(ctx context.Context, products []domain.Product) {
	if len(products) == 0 {
		return
	}

	windows, err := s.clickWindowsByProduct(ctx, time.Now().UTC(), "")
	if err != nil {
		log.Printf("click window: %v", err)
		return
	}

	for i := range products {
		applyClickWindow(&products[i], windows[products[i].ID])
	}
}

func (s *Store) applyClickWindowOne(ctx context.Context, product *domain.Product) {
	windows, err := s.clickWindowsByProduct(ctx, time.Now().UTC(), product.ID)
	if err != nil {
		log.Printf("click window %s: %v", product.ID, err)
		return
	}
	applyClickWindow(product, windows[product.ID])
}
