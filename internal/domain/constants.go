package domain

import "time"

const (
	MinNewBidCents      = 0
	MinRaiseCents       = 100
	TakeFirstExtraCents = 100
	MaxBidCents         = 99_999_900
	MaxCategories       = 15
	ClickWindow         = time.Hour
)

// LaunchDate is when public visit counting started. The total only goes up.
var LaunchDate = time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)

