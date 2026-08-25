package domain

import (
	"sort"
	"time"
)

type RankRange string

const (
	RankDaily   RankRange = "daily"
	RankWeekly  RankRange = "weekly"
	RankMonthly RankRange = "monthly"
	RankAll     RankRange = "all"
)

const (
	RankWindowDaily   = 24 * time.Hour
	RankWindowWeekly  = 7 * 24 * time.Hour
	RankWindowMonthly = 30 * 24 * time.Hour
)

func SortByBid(products []Product) []Product {
	out := append([]Product(nil), products...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].BidCents != out[j].BidCents {
			return out[i].BidCents > out[j].BidCents
		}
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func PaidCents(product Product, rng RankRange) int {
	switch rng {
	case RankDaily:
		return product.PaidDailyCents
	case RankWeekly:
		return product.PaidWeeklyCents
	case RankMonthly:
		return product.PaidMonthlyCents
	default:
		if product.PaidAllTimeCents > product.BidCents {
			return product.PaidAllTimeCents
		}
		return product.BidCents
	}
}

func ClicksForRange(product Product, rng RankRange) int {
	switch rng {
	case RankDaily:
		return product.ClicksDaily
	case RankWeekly:
		return product.ClicksWeekly
	case RankMonthly:
		return product.ClicksMonthly
	default:
		if product.ClicksAllTime > product.Clicks {
			return product.ClicksAllTime
		}
		return product.Clicks
	}
}

func ClickDedupKey(productID, visitorID string, now time.Time) string {
	return productID + ":" + visitorID + ":" + now.UTC().Truncate(ClickWindow).Format("2006-01-02T15")
}

func SortByPaid(products []Product, rng RankRange) []Product {
	out := append([]Product(nil), products...)
	sort.SliceStable(out, func(i, j int) bool {
		left := PaidCents(out[i], rng)
		right := PaidCents(out[j], rng)
		if left != right {
			return left > right
		}
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID < out[j].ID
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func RankOf(productID string, products []Product) int {
	return RankOfRange(productID, products, RankAll)
}

func RankOfRange(productID string, products []Product, rng RankRange) int {
	for i, product := range SortByPaid(products, rng) {
		if product.ID == productID {
			return i + 1
		}
	}
	return 0
}

func ValidateBid(amountCents int, existing *Product) (paidCents int, err error) {
	if amountCents%100 != 0 {
		return 0, ErrWholeDollars
	}
	if amountCents > MaxBidCents {
		return 0, ErrBidTooHigh
	}
	if amountCents < 0 {
		return 0, ErrNeedOneDollar
	}

	if existing != nil && amountCents < MinRaiseCents {
		return 0, RaiseError(MinRaiseCents)
	}

	return amountCents, nil
}

func ValidateCategories(slugs []string) ([]string, error) {
	seen := make(map[string]struct{}, len(slugs))
	out := make([]string, 0, len(slugs))
	for _, slug := range slugs {
		if slug == "" {
			continue
		}
		if !KnownCategory(slug) {
			return nil, ErrUnknownCat
		}
		if _, ok := seen[slug]; ok {
			continue
		}
		seen[slug] = struct{}{}
		out = append(out, slug)
	}

	if len(out) == 0 {
		return nil, ErrNeedCategory
	}
	if len(out) > MaxCategories {
		return nil, ErrTooManyCats
	}
	return out, nil
}

func NextBidAfterPayment(existing *Product, _, paidCents int) int {
	if existing == nil {
		return paidCents
	}
	return existing.BidCents + paidCents
}
