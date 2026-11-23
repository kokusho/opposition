package pgn

import (
	"testing"

	"opposition/internal/core"
)

func TestParseResultMapsThePGNResultTag(t *testing.T) {
	cases := map[string]core.Result{
		"1-0":     core.WhiteWon,
		"0-1":     core.BlackWon,
		"1/2-1/2": core.Draw,
		"*":       core.ResultUnknown,
		"":        core.ResultUnknown,
		"+/-":     core.ResultUnknown,
	}
	for tag, want := range cases {
		if got := parseResult(tag); got != want {
			t.Errorf("parseResult(%q) = %v, want %v", tag, got, want)
		}
	}
}
