package domain

import (
	"errors"
	"net/url"
	"strings"
)

var (
	ErrEmptyTarget   = errors.New("Drop in a website.")
	ErrNeedRealSite  = errors.New("Need a real site (with a dot).")
	ErrUnreadable    = errors.New("Can't make sense of that. Try a website.")
	ErrWholeDollars  = errors.New("Whole dollars only, no cents.")
	ErrBidTooHigh    = errors.New("Whoa, $999,999 is as high as it goes.")
	ErrNeedOneDollar = errors.New("Payments start at $0.")
	ErrTooManyCats   = errors.New("15 categories is the max.")
	ErrNeedCategory  = errors.New("Pick at least one category.")
	ErrUnknownCat    = errors.New("One of those categories isn't on the list.")
)

func ParseListingTarget(raw string) (ListingTarget, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ListingTarget{}, ErrEmptyTarget
	}

	if strings.HasPrefix(trimmed, "@") {
		return ListingTarget{}, ErrNeedRealSite
	}

	withProtocol := trimmed
	if !strings.HasPrefix(strings.ToLower(trimmed), "http://") &&
		!strings.HasPrefix(strings.ToLower(trimmed), "https://") {
		withProtocol = "https://" + trimmed
	}

	parsed, err := url.Parse(withProtocol)
	if err != nil || parsed.Host == "" {
		return ListingTarget{}, ErrUnreadable
	}

	host := strings.ToLower(parsed.Hostname())
	host = strings.TrimPrefix(host, "www.")
	if !strings.Contains(host, ".") {
		return ListingTarget{}, ErrNeedRealSite
	}

	path := strings.ToLower(strings.TrimRight(parsed.Path, "/"))
	return ListingTarget{
		Key:        host + path,
		WebsiteURL: "https://" + host + path,
		Name:       host,
	}, nil
}

func RaiseError(minCents int) error {
	return &BidTooLowError{MinCents: minCents}
}

type BidTooLowError struct {
	MinCents int
}

func (e *BidTooLowError) Error() string {
	return "Pay at least $" + dollars(e.MinCents) + " to move up."
}

func dollars(cents int) string {
	return itoa(cents / 100)
}
