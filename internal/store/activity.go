package store

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"saaslb-backend/internal/domain"
)

func (s *Store) ListRecentActivity(ctx context.Context, limit int) ([]domain.ActivityEvent, error) {
	limit = domain.ClampActivityLimit(limit)

	clicks, err := s.recentClickDocs(ctx, limit)
	if err != nil {
		return nil, err
	}
	checkouts, err := s.recentPaidCheckoutDocs(ctx, limit)
	if err != nil {
		return nil, err
	}

	ids := make([]string, 0, len(clicks)+len(checkouts))
	for _, click := range clicks {
		ids = append(ids, click.ProductID)
	}
	for _, checkout := range checkouts {
		if checkout.ProductID != nil && *checkout.ProductID != "" {
			ids = append(ids, *checkout.ProductID)
		}
	}

	products, err := s.productDocsByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	events := make([]domain.ActivityEvent, 0, len(clicks)+len(checkouts))
	for _, click := range clicks {
		product, ok := activityProduct(products, click.ProductID)
		if !ok {
			continue
		}
		events = append(events, domain.ActivityEvent{
			ID:      click.ID,
			Kind:    domain.ActivityClick,
			At:      click.CreatedAt,
			Product: product,
		})
	}
	for _, checkout := range checkouts {
		productID := ""
		if checkout.ProductID != nil {
			productID = *checkout.ProductID
		}
		product, ok := activityProduct(products, productID)
		if !ok {
			product = domain.ActivityProduct{
				ID:         productID,
				Name:       checkout.Name,
				WebsiteURL: checkout.WebsiteURL,
				ListingKey: checkout.ListingKey,
			}
			if product.ID == "" || product.Name == "" {
				continue
			}
		}

		event := domain.ActivityEvent{
			ID:      checkout.ID,
			Kind:    domain.ActivityKindForPayment(checkout.PaidCents),
			At:      checkoutActivityAt(checkout),
			Product: product,
		}
		if event.Kind == domain.ActivityPaid {
			event.PaidCents = checkout.PaidCents
		}
		events = append(events, event)
	}

	return domain.SortRecentActivity(events, limit), nil
}

func (s *Store) recentClickDocs(ctx context.Context, limit int) ([]clickDoc, error) {
	cursor, err := s.clicks().Find(ctx, bson.M{}, options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []clickDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func (s *Store) recentPaidCheckoutDocs(ctx context.Context, limit int) ([]checkoutDoc, error) {
	cursor, err := s.checkouts().Find(ctx, bson.M{"status": "paid"}, options.Find().
		SetSort(bson.D{
			{Key: "paid_at", Value: -1},
			{Key: "created_at", Value: -1},
		}).
		SetLimit(int64(limit)))
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []checkoutDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	return docs, nil
}

func (s *Store) productDocsByIDs(ctx context.Context, ids []string) (map[string]productDoc, error) {
	unique := uniqueNonEmpty(ids)
	out := make(map[string]productDoc, len(unique))
	if len(unique) == 0 {
		return out, nil
	}

	cursor, err := s.products().Find(ctx, bson.M{"_id": bson.M{"$in": unique}})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var docs []productDoc
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, err
	}
	for _, doc := range docs {
		out[doc.ID] = doc
	}
	return out, nil
}

func activityProduct(products map[string]productDoc, id string) (domain.ActivityProduct, bool) {
	doc, ok := products[id]
	if !ok {
		return domain.ActivityProduct{}, false
	}
	return domain.ActivityProduct{
		ID:         doc.ID,
		Name:       doc.Name,
		WebsiteURL: doc.WebsiteURL,
		IconURL:    doc.IconURL,
		ListingKey: doc.ListingKey,
	}, true
}

func checkoutActivityAt(checkout checkoutDoc) time.Time {
	if checkout.PaidAt != nil && !checkout.PaidAt.IsZero() {
		return *checkout.PaidAt
	}
	return checkout.CreatedAt
}

func uniqueNonEmpty(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}
