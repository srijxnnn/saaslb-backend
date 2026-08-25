package domain

import (
	"strings"
	"time"
	"unicode"
)

type Category struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

type Product struct {
	ID               string     `json:"id"`
	Slug             string     `json:"slug"`
	Name             string     `json:"name"`
	Tagline          string     `json:"tagline"`
	WebsiteURL       string     `json:"websiteUrl"`
	IconURL          string     `json:"iconUrl"`
	ListingKey       string     `json:"listingKey"`
	Categories       []string   `json:"categories"`
	BidCents         int        `json:"bidCents"`
	PaidDailyCents   int        `json:"paidDailyCents"`
	PaidWeeklyCents  int        `json:"paidWeeklyCents"`
	PaidMonthlyCents int        `json:"paidMonthlyCents"`
	PaidAllTimeCents int        `json:"paidAllTimeCents"`
	Clicks           int        `json:"clicks"`
	ClicksLastHour   int        `json:"clicksLastHour"`
	ClicksDaily      int        `json:"clicksDaily"`
	ClicksWeekly     int        `json:"clicksWeekly"`
	ClicksMonthly    int        `json:"clicksMonthly"`
	ClicksAllTime    int        `json:"clicksAllTime"`
	CreatedAt        time.Time  `json:"createdAt"`
	UpdatedAt        time.Time  `json:"updatedAt"`
	LastPaidAt       *time.Time `json:"lastPaidAt,omitempty"`
	LastPaidCents    int        `json:"lastPaidCents"`
	Period           string     `json:"-"`
}

type ListingTarget struct {
	Key        string
	WebsiteURL string
	Name       string
}

type PayInput struct {
	Target      string
	AmountCents int
	Tagline     string
	Categories  []string
}

func Slugify(value string) string {
	var b strings.Builder
	lastDash := true

	for _, r := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}

	slug := strings.Trim(b.String(), "-")
	if slug == "" {
		return "listing"
	}
	return slug
}

func UniqueSlug(name string, taken map[string]struct{}) string {
	base := Slugify(name)
	slug := base
	n := 2
	for {
		if _, exists := taken[slug]; !exists {
			return slug
		}
		slug = base + "-" + itoa(n)
		n++
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}
