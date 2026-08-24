package domain

import "sort"

func SortByBid(products []Product) []Product {
	out := append([]Product(nil), products...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].BidCents != out[j].BidCents {
			return out[i].BidCents > out[j].BidCents
		}
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out
}

func RankOf(productID string, products []Product) int {
	for i, product := range SortByBid(products) {
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

	if existing != nil {
		min := existing.BidCents + MinRaiseCents
		if amountCents < min {
			return 0, RaiseError(min)
		}
		return amountCents - existing.BidCents, nil
	}

	if amountCents < 0 {
		return 0, ErrNeedOneDollar
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

func NextBidAfterPayment(existing *Product, targetCents, paidCents int) int {
	if existing == nil {
		return targetCents
	}
	if targetCents >= existing.BidCents+MinRaiseCents {
		return targetCents
	}
	return existing.BidCents + paidCents
}
