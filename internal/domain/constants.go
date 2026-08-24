package domain

import "time"

const (
	MinNewBidCents      = 0
	MinRaiseCents       = 100
	TakeFirstExtraCents = 100
	MaxBidCents         = 99_999_900
	MaxCategories       = 15
)

// LaunchDate is when public visit counting started. The total only goes up.
var LaunchDate = time.Date(2026, time.August, 23, 0, 0, 0, 0, time.UTC)

var AccentPalette = []string{
	"#0f766e",
	"#6d28d9",
	"#0369a1",
	"#b45309",
	"#be123c",
	"#0e7490",
	"#4338ca",
	"#c2410c",
	"#4d7c0f",
	"#a21caf",
	"#0f766e",
	"#1d4ed8",
}
