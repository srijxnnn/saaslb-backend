package domain

import (
	"sort"
	"time"
)

const (
	ActivityClick  = "click"
	ActivityPaid   = "paid"
	ActivityListed = "listed"
)

const (
	ActivityDefaultLimit = 5
	ActivityMaxLimit     = 50
)

type ActivityProduct struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	WebsiteURL string `json:"websiteUrl"`
	IconURL    string `json:"iconUrl,omitempty"`
	ListingKey string `json:"listingKey"`
}

type ActivityEvent struct {
	ID        string          `json:"id"`
	Kind      string          `json:"kind"`
	At        time.Time       `json:"at"`
	PaidCents int             `json:"paidCents,omitempty"`
	Product   ActivityProduct `json:"product"`
}

// ActivityKindForPayment is "just signed up" when nothing was charged, and
// "paid $N" for any real payment — including the first listing payment.
func ActivityKindForPayment(paidCents int) string {
	if paidCents <= 0 {
		return ActivityListed
	}
	return ActivityPaid
}

func ClampActivityLimit(limit int) int {
	if limit <= 0 {
		return ActivityDefaultLimit
	}
	if limit > ActivityMaxLimit {
		return ActivityMaxLimit
	}
	return limit
}

// SortRecentActivity newest-first. Equal timestamps break ties on id so the
// order stays stable across refreshes.
func SortRecentActivity(events []ActivityEvent, limit int) []ActivityEvent {
	out := append([]ActivityEvent(nil), events...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].At.Equal(out[j].At) {
			return out[i].ID > out[j].ID
		}
		return out[i].At.After(out[j].At)
	})

	limit = ClampActivityLimit(limit)
	if len(out) > limit {
		return out[:limit]
	}
	return out
}
