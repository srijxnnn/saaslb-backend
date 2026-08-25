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

func TestSortByPaidWindows(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	products := []Product{
		{ID: "old", PaidAllTimeCents: 500, PaidMonthlyCents: 0, PaidWeeklyCents: 0, PaidDailyCents: 0, CreatedAt: now},
		{ID: "hot", PaidAllTimeCents: 200, PaidMonthlyCents: 200, PaidWeeklyCents: 200, PaidDailyCents: 150, CreatedAt: now.Add(time.Hour)},
		{ID: "mid", PaidAllTimeCents: 300, PaidMonthlyCents: 300, PaidWeeklyCents: 50, PaidDailyCents: 0, CreatedAt: now.Add(2 * time.Hour)},
	}

	daily := SortByPaid(products, RankDaily)
	if daily[0].ID != "hot" || daily[1].ID != "old" || daily[2].ID != "mid" {
		t.Fatalf("daily = %#v", []string{daily[0].ID, daily[1].ID, daily[2].ID})
	}

	weekly := SortByPaid(products, RankWeekly)
	if weekly[0].ID != "hot" || weekly[1].ID != "mid" || weekly[2].ID != "old" {
		t.Fatalf("weekly = %#v", []string{weekly[0].ID, weekly[1].ID, weekly[2].ID})
	}

	monthly := SortByPaid(products, RankMonthly)
	if monthly[0].ID != "mid" || monthly[1].ID != "hot" || monthly[2].ID != "old" {
		t.Fatalf("monthly = %#v", []string{monthly[0].ID, monthly[1].ID, monthly[2].ID})
	}

	all := SortByPaid(products, RankAll)
	if all[0].ID != "old" || all[1].ID != "mid" || all[2].ID != "hot" {
		t.Fatalf("all = %#v", []string{all[0].ID, all[1].ID, all[2].ID})
	}
	if RankOfRange("hot", products, RankDaily) != 1 {
		t.Fatalf("daily rank of hot = %d", RankOfRange("hot", products, RankDaily))
	}
}

func TestEqualPaidKeepsEarlierListingAhead(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	products := []Product{
		{ID: "new", PaidAllTimeCents: 500, PaidMonthlyCents: 500, CreatedAt: now.Add(time.Hour)},
		{ID: "old", PaidAllTimeCents: 500, PaidMonthlyCents: 500, CreatedAt: now},
	}

	if RankOfRange("new", products, RankMonthly) != 2 {
		t.Fatalf("new listing should be #2, got %d", RankOfRange("new", products, RankMonthly))
	}
	if RankOfRange("old", products, RankMonthly) != 1 {
		t.Fatalf("old listing should be #1, got %d", RankOfRange("old", products, RankMonthly))
	}
}

func TestClicksForRange(t *testing.T) {
	t.Parallel()

	product := Product{
		Clicks:        4,
		ClicksDaily:   1,
		ClicksWeekly:  2,
		ClicksMonthly: 3,
		ClicksAllTime: 9,
	}
	if got := ClicksForRange(product, RankDaily); got != 1 {
		t.Fatalf("daily = %d", got)
	}
	if got := ClicksForRange(product, RankWeekly); got != 2 {
		t.Fatalf("weekly = %d", got)
	}
	if got := ClicksForRange(product, RankMonthly); got != 3 {
		t.Fatalf("monthly = %d", got)
	}
	if got := ClicksForRange(product, RankAll); got != 9 {
		t.Fatalf("all = %d", got)
	}
}

func TestNextBidAfterPayment(t *testing.T) {
	t.Parallel()

	existing := &Product{BidCents: 1000}
	if got := NextBidAfterPayment(existing, 0, 500); got != 1500 {
		t.Fatalf("got %d", got)
	}
	if got := NextBidAfterPayment(nil, 0, 200); got != 200 {
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
