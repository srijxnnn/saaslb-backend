package store

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"saaslb-backend/internal/domain"
	"saaslb-backend/internal/metadesc"
)

var ErrNotFound = errors.New("not found")
var ErrAlreadyProcessed = errors.New("already processed")
var ErrMetaCooldown = errors.New("already refreshed that listing a moment ago")
var ErrSiteUnreadable = errors.New("could not read that site")

type Checkout struct {
	ID                string
	SessionID         *string
	PaymentID         *string
	ListingKey        string
	WebsiteURL        string
	Name              string
	Tagline           string
	IconURL           string
	Categories        []string
	AmountCents       int
	PaidCents         int
	ExistingProductID *string
	Status            string
	CreatedAt         time.Time
	PaidAt            *time.Time
	ProductID         *string
}

func NewID(prefix string) string {
	var b [10]byte
	_, _ = rand.Read(b[:])
	return prefix + hex.EncodeToString(b[:])
}

func (s *Store) CreateCheckout(ctx context.Context, checkout Checkout) error {
	_, err := s.checkouts().InsertOne(ctx, checkoutFromStore(checkout))
	return err
}

func (s *Store) SetCheckoutSession(ctx context.Context, id, sessionID string) error {
	_, err := s.checkouts().UpdateOne(ctx, bson.M{"_id": id}, bson.M{"$set": bson.M{"session_id": sessionID}})
	return err
}

func (s *Store) GetCheckout(ctx context.Context, id string) (Checkout, error) {
	return s.findCheckout(ctx, bson.M{"_id": id})
}

func (s *Store) CheckoutBySession(ctx context.Context, sessionID string) (Checkout, error) {
	return s.findCheckout(ctx, bson.M{"session_id": sessionID})
}

func (s *Store) CheckoutByPayment(ctx context.Context, paymentID string) (Checkout, error) {
	return s.findCheckout(ctx, bson.M{"payment_id": paymentID})
}

func (s *Store) MarkCheckoutFailed(ctx context.Context, id, paymentID string) error {
	set := bson.M{"status": "failed"}
	if paymentID != "" {
		set["payment_id"] = paymentID
	}
	_, err := s.checkouts().UpdateOne(ctx, bson.M{"_id": id, "status": "pending"}, bson.M{"$set": set})
	return err
}

func (s *Store) FulfillCheckout(ctx context.Context, checkoutID, paymentID string) (domain.Product, error) {
	checkout, err := s.GetCheckout(ctx, checkoutID)
	if err != nil {
		return domain.Product{}, err
	}
	if checkout.Status == "paid" && checkout.ProductID != nil {
		product, err := s.GetProduct(ctx, *checkout.ProductID)
		if err != nil {
			return domain.Product{}, err
		}
		return product, ErrAlreadyProcessed
	}
	if checkout.Status != "pending" && checkout.Status != "paid" {
		return domain.Product{}, fmt.Errorf("checkout is %s", checkout.Status)
	}

	existing, err := s.findProductDoc(ctx, bson.M{"listing_key": checkout.ListingKey})
	if err != nil && !errors.Is(err, ErrNotFound) {
		return domain.Product{}, err
	}

	info, fetchErr := metadesc.Fetch(ctx, checkout.WebsiteURL)
	if fetchErr != nil {
		log.Printf("listing meta %s: %v", checkout.WebsiteURL, fetchErr)
	}
	if info.Tagline != "" {
		checkout.Tagline = info.Tagline
	}
	if info.IconURL != "" {
		checkout.IconURL = info.IconURL
	}

	var product domain.Product
	now := time.Now().UTC()

	if errors.Is(err, ErrNotFound) {
		product, err = s.createFromCheckout(ctx, checkout, now)
		if err != nil {
			return domain.Product{}, err
		}
	} else {
		product, err = s.raiseProduct(ctx, existing, checkout)
		if err != nil {
			return domain.Product{}, err
		}
	}

	set := bson.M{
		"status":     "paid",
		"paid_at":    now,
		"product_id": product.ID,
	}
	if checkout.Tagline != "" {
		set["tagline"] = checkout.Tagline
	}
	if checkout.IconURL != "" {
		set["icon_url"] = checkout.IconURL
	}
	if paymentID != "" {
		set["payment_id"] = paymentID
	}
	if _, err := s.checkouts().UpdateOne(ctx, bson.M{"_id": checkout.ID}, bson.M{"$set": set}); err != nil {
		return domain.Product{}, err
	}
	return product, nil
}

func (s *Store) createFromCheckout(ctx context.Context, checkout Checkout, now time.Time) (domain.Product, error) {
	taken, err := s.takenSlugs(ctx)
	if err != nil {
		return domain.Product{}, err
	}

	slug := domain.UniqueSlug(checkout.Name, taken)
	product := domain.Product{
		ID:         "prd_" + slug,
		Slug:       slug,
		Name:       checkout.Name,
		Tagline:    checkout.Tagline,
		WebsiteURL: checkout.WebsiteURL,
		IconURL:    checkout.IconURL,
		ListingKey: checkout.ListingKey,
		Categories: checkout.Categories,
		BidCents:   domain.NextBidAfterPayment(nil, checkout.AmountCents, checkout.PaidCents),
		Clicks:     0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	err = s.insertProduct(ctx, product, checkout.ID)
	if err == nil {
		return product, nil
	}
	if !mongo.IsDuplicateKeyError(err) {
		return domain.Product{}, err
	}

	existing, err := s.findProductDoc(ctx, bson.M{"listing_key": checkout.ListingKey})
	if err != nil {
		return domain.Product{}, err
	}
	return s.raiseProduct(ctx, existing, checkout)
}

// raiseProduct applies a payment with compare-and-set on bid_cents so two
// concurrent checkouts cannot both add from the same stale total. last_checkout_id
// makes a webhook retry a no-op instead of a second payment.
func (s *Store) raiseProduct(ctx context.Context, existing productDoc, checkout Checkout) (domain.Product, error) {
	for {
		if existing.LastCheckoutID == checkout.ID {
			return existing.toDomain(), nil
		}

		next := existing.toDomain()
		next.BidCents = domain.NextBidAfterPayment(&next, checkout.AmountCents, checkout.PaidCents)

		set := bson.M{
			"bid_cents":        next.BidCents,
			"last_checkout_id": checkout.ID,
			"updated_at":       time.Now().UTC(),
		}
		if len(checkout.Categories) > 0 {
			set["categories"] = checkout.Categories
		}
		if checkout.Tagline != "" {
			set["tagline"] = checkout.Tagline
		}
		if checkout.IconURL != "" {
			set["icon_url"] = checkout.IconURL
		}

		err := s.products().FindOneAndUpdate(ctx,
			bson.M{
				"_id":              existing.ID,
				"bid_cents":        existing.BidCents,
				"last_checkout_id": bson.M{"$ne": checkout.ID},
			},
			bson.M{"$set": set},
			options.FindOneAndUpdate().SetReturnDocument(options.After),
		).Decode(&existing)
		if err == nil {
			return existing.toDomain(), nil
		}
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return domain.Product{}, err
		}

		existing, err = s.findProductDoc(ctx, bson.M{"_id": existing.ID})
		if err != nil {
			return domain.Product{}, err
		}
	}
}

func (s *Store) findCheckout(ctx context.Context, filter bson.M) (Checkout, error) {
	var doc checkoutDoc
	err := s.checkouts().FindOne(ctx, filter).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return Checkout{}, ErrNotFound
	}
	if err != nil {
		return Checkout{}, err
	}
	return doc.toStore(), nil
}
