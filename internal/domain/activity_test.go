package domain

import (
	"testing"
	"time"
)

func TestActivityKindForPayment(t *testing.T) {
	t.Parallel()

	if got := ActivityKindForPayment(0); got != ActivityListed {
		t.Fatalf("free listing: got %q", got)
	}
	if got := ActivityKindForPayment(500); got != ActivityPaid {
		t.Fatalf("paid listing: got %q", got)
	}
}

func TestSortRecentActivity(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	events := []ActivityEvent{
		{ID: "listed", Kind: ActivityListed, At: now.Add(-3 * time.Minute)},
		{ID: "paid", Kind: ActivityPaid, At: now.Add(-2 * time.Minute), PaidCents: 500},
		{ID: "click-old", Kind: ActivityClick, At: now.Add(-4 * time.Minute)},
		{ID: "click-new", Kind: ActivityClick, At: now.Add(-time.Minute)},
	}

	got := SortRecentActivity(events, 3)
	if len(got) != 3 {
		t.Fatalf("len = %d, want 3", len(got))
	}
	if got[0].ID != "click-new" || got[1].ID != "paid" || got[2].ID != "listed" {
		t.Fatalf("order = %q %q %q", got[0].ID, got[1].ID, got[2].ID)
	}
}

func TestSortRecentActivityTieBreak(t *testing.T) {
	t.Parallel()

	at := time.Date(2026, 8, 25, 12, 0, 0, 0, time.UTC)
	got := SortRecentActivity([]ActivityEvent{
		{ID: "clk_aaa", At: at},
		{ID: "clk_zzz", At: at},
	}, 5)

	if got[0].ID != "clk_zzz" {
		t.Fatalf("tie break: got %q", got[0].ID)
	}
}
