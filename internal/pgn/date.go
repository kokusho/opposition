package pgn

import (
	"strconv"
	"strings"
	"time"
)

// parseDate reads a PGN Date tag ("2026.03.14"). An unknown month or day
// ("??") becomes the first of the period. An unknown or unreadable date is
// the zero time.
func parseDate(tag string) time.Time {
	parts := strings.Split(strings.TrimSpace(tag), ".")
	if len(parts) > 3 {
		return time.Time{}
	}

	year, err := strconv.Atoi(parts[0])
	if err != nil || len(parts[0]) != 4 {
		return time.Time{}
	}
	month, day := 1, 1
	if len(parts) > 1 {
		if month, err = knownOr(parts[1], 1); err != nil {
			return time.Time{}
		}
	}
	if len(parts) > 2 {
		if day, err = knownOr(parts[2], 1); err != nil {
			return time.Time{}
		}
	}

	date := time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
	if date.Month() != time.Month(month) || date.Day() != day {
		return time.Time{} // out of range, e.g. month 13 or day 40
	}
	return date
}

// knownOr parses a date part, returning fallback for PGN's "??".
func knownOr(part string, fallback int) (int, error) {
	if strings.Trim(part, "?") == "" {
		return fallback, nil
	}
	return strconv.Atoi(part)
}
