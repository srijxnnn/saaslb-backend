package store

import (
	"time"

	"saaslb-backend/internal/domain"
)

// productDoc is the MongoDB shape of a listing. Domain.Product stays JSON-facing
// so HTTP handlers do not need bson tags.
type productDoc struct {
	ID              string    `bson:"_id"`
	Slug            string    `bson:"slug"`
	Name            string    `bson:"name"`
	Tagline         string    `bson:"tagline"`
	WebsiteURL      string    `bson:"website_url"`
	IconURL         string    `bson:"icon_url,omitempty"`
	ListingKey      string    `bson:"listing_key"`
	Categories      []string  `bson:"categories"`
	BidCents        int       `bson:"bid_cents"`
	Clicks          int       `bson:"clicks"`
	CreatedAt       time.Time `bson:"created_at"`
	Accent          string    `bson:"accent"`
	Period          string    `bson:"period"`
	LastCheckoutID  string    `bson:"last_checkout_id,omitempty"`
	MetaRefreshedAt time.Time `bson:"meta_refreshed_at,omitempty"`
}

type checkoutDoc struct {
	ID                string     `bson:"_id"`
	SessionID         *string    `bson:"session_id,omitempty"`
	PaymentID         *string    `bson:"payment_id,omitempty"`
	ListingKey        string     `bson:"listing_key"`
	WebsiteURL        string     `bson:"website_url"`
	Name              string     `bson:"name"`
	Tagline           string     `bson:"tagline"`
	Categories        []string   `bson:"categories"`
	AmountCents       int        `bson:"amount_cents"`
	PaidCents         int        `bson:"paid_cents"`
	ExistingProductID *string    `bson:"existing_product_id,omitempty"`
	Status            string     `bson:"status"`
	CreatedAt         time.Time  `bson:"created_at"`
	PaidAt            *time.Time `bson:"paid_at,omitempty"`
	ProductID         *string    `bson:"product_id,omitempty"`
}

func (d productDoc) toDomain() domain.Product {
	return domain.Product{
		ID:         d.ID,
		Slug:       d.Slug,
		Name:       d.Name,
		Tagline:    d.Tagline,
		WebsiteURL: d.WebsiteURL,
		IconURL:    d.IconURL,
		ListingKey: d.ListingKey,
		Categories: nonNil(d.Categories),
		BidCents:   d.BidCents,
		Clicks:     d.Clicks,
		CreatedAt:  d.CreatedAt,
		Accent:     d.Accent,
		Period:     d.Period,
	}
}

func (d checkoutDoc) toStore() Checkout {
	return Checkout{
		ID:                d.ID,
		SessionID:         d.SessionID,
		PaymentID:         d.PaymentID,
		ListingKey:        d.ListingKey,
		WebsiteURL:        d.WebsiteURL,
		Name:              d.Name,
		Tagline:           d.Tagline,
		Categories:        nonNil(d.Categories),
		AmountCents:       d.AmountCents,
		PaidCents:         d.PaidCents,
		ExistingProductID: d.ExistingProductID,
		Status:            d.Status,
		CreatedAt:         d.CreatedAt,
		PaidAt:            d.PaidAt,
		ProductID:         d.ProductID,
	}
}

func productFromDomain(product domain.Product, lastCheckoutID string) productDoc {
	return productDoc{
		ID:             product.ID,
		Slug:           product.Slug,
		Name:           product.Name,
		Tagline:        product.Tagline,
		WebsiteURL:     product.WebsiteURL,
		IconURL:        product.IconURL,
		ListingKey:     product.ListingKey,
		Categories:     nonNil(product.Categories),
		BidCents:       product.BidCents,
		Clicks:         product.Clicks,
		CreatedAt:      product.CreatedAt,
		Accent:         product.Accent,
		Period:         product.Period,
		LastCheckoutID: lastCheckoutID,
	}
}

func checkoutFromStore(checkout Checkout) checkoutDoc {
	return checkoutDoc{
		ID:                checkout.ID,
		SessionID:         checkout.SessionID,
		PaymentID:         checkout.PaymentID,
		ListingKey:        checkout.ListingKey,
		WebsiteURL:        checkout.WebsiteURL,
		Name:              checkout.Name,
		Tagline:           checkout.Tagline,
		Categories:        nonNil(checkout.Categories),
		AmountCents:       checkout.AmountCents,
		PaidCents:         checkout.PaidCents,
		ExistingProductID: checkout.ExistingProductID,
		Status:            checkout.Status,
		CreatedAt:         checkout.CreatedAt,
		PaidAt:            checkout.PaidAt,
		ProductID:         checkout.ProductID,
	}
}

func nonNil(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}
