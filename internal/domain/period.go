package domain

import (
	"fmt"
	"time"
)

// FastPeriodReset is a verification switch. When true, bids drop back to $1
// every minute instead of at the start of each calendar month. Flip this back
// to false after you have confirmed the rollover.
const FastPeriodReset = false

func CurrentPeriodKey(now time.Time) string {
	if FastPeriodReset {
		return now.Format("2006-01-02T15:04")
	}
	return fmt.Sprintf("%04d-%02d", now.Year(), int(now.Month()))
}

func NextMonthStart(now time.Time) time.Time {
	if FastPeriodReset {
		return now.Truncate(time.Minute).Add(time.Minute)
	}
	return time.Date(now.Year(), now.Month()+1, 1, 0, 0, 0, 0, now.Location())
}
