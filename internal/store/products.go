package store

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"saaslb-backend/internal/domain"
	"saaslb-backend/internal/metadesc"
)

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
	s.applyClickWindow(ctx, products)
	s.applyPaidWindows(ctx, products)
	return products, nil
}

func (s *Store) GetProduct(ctx context.Context, idOrSlug string) (domain.Product, error) {
	product, err := s.findProduct(ctx, bson.M{"$or": []bson.M{
		{"_id": idOrSlug},
		{"slug": idOrSlug},
	}})
	if err != nil {
		return domain.Product{}, err
	}
	s.applyClickWindowOne(ctx, &product)
	s.applyPaidWindowsOne(ctx, &product)
	return product, nil
}

func (s *Store) ProductByListingKey(ctx context.Context, key string) (domain.Product, error) {
	return s.findProduct(ctx, bson.M{"listing_key": key})
}

func (s *Store) IncrementClicks(ctx context.Context, id, visitorID string) (int, bool, error) {
	var current productDoc
	err := s.products().FindOne(ctx, bson.M{"_id": id}).Decode(&current)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, false, ErrNotFound
	}
	if err != nil {
		return 0, false, err
	}

	insertErr := s.recordClickEvent(ctx, id, visitorID)
	if mongo.IsDuplicateKeyError(insertErr) {
		return current.Clicks, false, nil
	}
	if insertErr != nil {
		return 0, false, insertErr
	}

	var doc productDoc
	err = s.products().FindOneAndUpdate(ctx,
		bson.M{"_id": id},
		bson.M{"$inc": bson.M{"clicks": 1}},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return 0, false, ErrNotFound
	}
	if err != nil {
		return 0, false, err
	}
	return doc.Clicks, true, nil
}

const metaRefreshCooldown = 10 * time.Second

func (s *Store) RefreshListingMeta(ctx context.Context, idOrSlug string) (domain.Product, error) {
	doc, err := s.findProductDoc(ctx, bson.M{"$or": []bson.M{
		{"_id": idOrSlug},
		{"slug": idOrSlug},
	}})
	if err != nil {
		return domain.Product{}, err
	}

	if !doc.MetaRefreshedAt.IsZero() && time.Since(doc.MetaRefreshedAt) < metaRefreshCooldown {
		return doc.toDomain(), ErrMetaCooldown
	}

	product := doc.toDomain()
	info, fetchErr := metadesc.FetchExact(ctx, product.WebsiteURL)
	if fetchErr != nil {
		log.Printf("listing meta refresh %s: %v", product.WebsiteURL, fetchErr)
	}

	now := time.Now().UTC()
	set := bson.M{
		"meta_refreshed_at": now,
		"updated_at":        now,
	}
	if info.Tagline != "" {
		set["tagline"] = info.Tagline
	}
	if info.IconURL != "" {
		set["icon_url"] = info.IconURL
	}

	if info.Tagline == "" && info.IconURL == "" {
		if fetchErr != nil {
			return product, ErrSiteUnreadable
		}
		s.applyClickWindowOne(ctx, &product)
		s.applyPaidWindowsOne(ctx, &product)
		return product, nil
	}

	var updated productDoc
	err = s.products().FindOneAndUpdate(ctx,
		bson.M{"_id": product.ID},
		bson.M{"$set": set},
		options.FindOneAndUpdate().SetReturnDocument(options.After),
	).Decode(&updated)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return domain.Product{}, ErrNotFound
	}
	if err != nil {
		return domain.Product{}, err
	}

	log.Printf("listing meta refresh %s: tagline=%q icon=%q", product.WebsiteURL, info.Tagline, info.IconURL)
	out := updated.toDomain()
	s.applyClickWindowOne(ctx, &out)
	s.applyPaidWindowsOne(ctx, &out)
	return out, nil
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
		set["updated_at"] = time.Now().UTC()

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
