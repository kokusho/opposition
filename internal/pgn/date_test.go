package pgn

import (
	"testing"
	"time"
)

func TestParseDateFillsUnknownPartsWithTheFirstOfThePeriod(t *testing.T) {
	day := func(y int, m time.Month, d int) time.Time { return time.Date(y, m, d, 0, 0, 0, 0, time.UTC) }
	cases := map[string]time.Time{
		"2026.03.14": day(2026, time.March, 14),
		"2019.11.??": day(2019, time.November, 1),
		"2019.??.??": day(2019, time.January, 1),
		"2019":       day(2019, time.January, 1),
	}
	for tag, want := range cases {
		if got := parseDate(tag); !got.Equal(want) {
			t.Errorf("parseDate(%q) = %v, want %v", tag, got, want)
		}
	}
}

func TestParseDateIsZeroWhenTheYearIsUnknown(t *testing.T) {
	for _, tag := range []string{"????.??.??", "", "?", "last spring", "2019.13.40"} {
		if got := parseDate(tag); !got.IsZero() {
			t.Errorf("parseDate(%q) = %v, want zero", tag, got)
		}
	}
}
