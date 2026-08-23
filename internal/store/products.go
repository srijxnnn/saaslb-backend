package store

import (
	"context"
	"errors"
	"log"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"saaslb-backend/internal/domain"
	"saaslb-backend/internal/metadesc"
)

func (s *Store) EnsurePeriod(ctx context.Context, period string) error {
	var current struct {
		Value string `bson:"value"`
	}
	err := s.meta().FindOne(ctx, bson.M{"_id": "period"}).Decode(&current)
	if errors.Is(err, mongo.ErrNoDocuments) {
		_, err = s.meta().InsertOne(ctx, bson.M{"_id": "period", "value": period})
		if err == nil {
			return nil
		}
		if !mongo.IsDuplicateKeyError(err) {
			return err
		}
		if err := s.meta().FindOne(ctx, bson.M{"_id": "period"}).Decode(&current); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	if current.Value == period {
		return nil
	}

	log.Printf("period rolled %s -> %s; bids reset to $1", current.Value, period)

	if _, err := s.products().UpdateMany(ctx,
		bson.M{"period": bson.M{"$ne": period}},
		bson.M{"$set": bson.M{"bid_cents": domain.MinNewBidCents, "period": period}},
	); err != nil {
		return err
	}

	_, err = s.meta().UpdateOne(ctx, bson.M{"_id": "period"}, bson.M{"$set": bson.M{"value": period}})
	return err
}

func (s *Store) ListProducts(ctx context.Context) ([]domain.Product, error) {
	opts := options.Find().SetSort(bson.D{
		{Key: "bid_cents", Value: -1},
		{Key: "created_at", Value: 1},
	})
	cursor, err := s.products().Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []productDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}

	products := make([]domain.Product, 0, len(docs))
	for _, doc := range docs {
		products = append(products, doc.toDomain())
	}
	return products, nil
}

func (s *Store) GetProduct(ctx context.Context, idOrSlug string) (domain.Product, error) {
	return s.findProduct(ctx, bson.M{"$or": []bson.M{
		{"_id": idOrSlug},
		{"slug": idOrSlug},
	}})
}

func (s *Store) ProductByListingKey(ctx context.Context, key string) (domain.Product, error) {
	return s.findProduct(ctx, bson.M{"listing_key": key})
}

func (s *Store) IncrementClicks(ctx context.Context, id string) (int, error) {
	var doc productDoc
	err := s.products().FindOneAndUpdate(ctx,
		bson.M{"_id": id},
		bson.M{"$inc": bson.M{"clicks": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, ErrNotFound
	}
	return doc.Clicks, err
}

func (s *Store) RefreshEmptyTaglines(ctx context.Context) error {
	products, err := s.ListProducts(ctx)
	if err != nil {
		return err
	}

	for _, product := range products {
		needTagline := strings.TrimSpace(product.Tagline) == ""
		needIcon := strings.TrimSpace(product.IconURL) == ""
		if !needTagline && !needIcon {
			continue
		}

		info, fetchErr := metadesc.Fetch(ctx, product.WebsiteURL)
		if fetchErr != nil {
			log.Printf("listing meta refresh %s: %v", product.WebsiteURL, fetchErr)
		}

		set := bson.M{}
		if needTagline && info.Tagline != "" {
			set["tagline"] = info.Tagline
		}
		if needIcon && info.IconURL != "" {
			set["icon_url"] = info.IconURL
		}
		if len(set) == 0 {
			continue
		}

		if _, err := s.products().UpdateOne(ctx, bson.M{"_id": product.ID}, bson.M{"$set": set}); err != nil {
			return err
		}
		log.Printf("listing meta refresh %s: tagline=%q icon=%q", product.WebsiteURL, info.Tagline, info.IconURL)
	}
	return nil
}

func (s *Store) insertProduct(ctx context.Context, product domain.Product, lastCheckoutID string) error {
	_, err := s.products().InsertOne(ctx, productFromDomain(product, lastCheckoutID))
	return err
}

func (s *Store) findProduct(ctx context.Context, filter bson.M) (domain.Product, error) {
	var doc productDoc
	err := s.products().FindOne(ctx, filter).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Product{}, ErrNotFound
	}
	if err != nil {
		return domain.Product{}, err
	}
	return doc.toDomain(), nil
}

func (s *Store) findProductDoc(ctx context.Context, filter bson.M) (productDoc, error) {
	var doc productDoc
	err := s.products().FindOne(ctx, filter).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return productDoc{}, ErrNotFound
	}
	return doc, err
}

func (s *Store) takenSlugs(ctx context.Context) (map[string]struct{}, error) {
	cursor, err := s.products().Find(ctx, bson.M{}, options.Find().SetProjection(bson.M{"slug": 1}))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	taken := map[string]struct{}{}
	for cursor.Next(ctx) {
		var doc struct {
			Slug string `bson:"slug"`
		}
		if err := cursor.Decode(&doc); err != nil {
			return nil, err
		}
		taken[doc.Slug] = struct{}{}
	}
	return taken, cursor.Err()
}
