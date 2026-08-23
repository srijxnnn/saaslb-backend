package domain

import (
	"testing"
	"time"
)

func TestSortAndRank(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	products := []Product{
		{ID: "b", BidCents: 200, CreatedAt: now.Add(time.Hour)},
		{ID: "a", BidCents: 200, CreatedAt: now},
		{ID: "c", BidCents: 100, CreatedAt: now},
	}

	sorted := SortByBid(products)
	if sorted[0].ID != "a" || sorted[1].ID != "b" || sorted[2].ID != "c" {
		t.Fatalf("order = %#v", []string{sorted[0].ID, sorted[1].ID, sorted[2].ID})
	}
	if RankOf("c", products) != 3 {
		t.Fatalf("rank of c = %d", RankOf("c", products))
	}
}

func TestNextBidAfterPayment(t *testing.T) {
	t.Parallel()

	existing := &Product{BidCents: 1000}
	if got := NextBidAfterPayment(existing, 1500, 500); got != 1500 {
		t.Fatalf("got %d", got)
	}
	if got := NextBidAfterPayment(existing, 1000, 500); got != 1500 {
		t.Fatalf("stale target should add paid cents, got %d", got)
	}
	if got := NextBidAfterPayment(nil, 200, 200); got != 200 {
		t.Fatalf("new listing got %d", got)
	}
}

func TestValidateCategories(t *testing.T) {
	t.Parallel()

	got, err := ValidateCategories([]string{"ai", "crm", "ai", ""})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "ai" || got[1] != "crm" {
		t.Fatalf("got %#v", got)
	}

	if _, err := ValidateCategories(nil); err != ErrNeedCategory {
		t.Fatalf("empty = %v", err)
	}
	if _, err := ValidateCategories([]string{"not-a-category"}); err != ErrUnknownCat {
		t.Fatalf("unknown = %v", err)
	}
}
