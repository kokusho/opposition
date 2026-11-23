package api

import (
	"testing"

	"opposition/internal/core"
)

func TestEvalDTOIsWrittenFromYourPointOfView(t *testing.T) {
	cases := []struct {
		name       string
		eval       core.Eval
		yourMove   bool
		text       string
		centipawns int
	}{
		{"you are better and to move", core.Eval{Centipawns: 31}, true, "+0.31", 31},
		{"the side to move is better and it is not you", core.Eval{Centipawns: 31}, false, "-0.31", -31},
		{"the side to move is worse and it is not you", core.Eval{Centipawns: -120}, false, "+1.20", 120},
		{"level", core.Eval{}, true, "0.00", 0},
		{"you mate in three", core.Eval{IsMate: true, MateIn: 3}, true, "#3", 1000},
		{"you get mated in three", core.Eval{IsMate: true, MateIn: 3}, false, "-#3", -1000},
		{"the opponent to move gets mated in two", core.Eval{IsMate: true, MateIn: -2}, false, "#2", 1000},
		{"the opponent to move is checkmated", core.Eval{IsMate: true, MateIn: 0}, false, "#", 1000},
		{"you are checkmated", core.Eval{IsMate: true, MateIn: 0}, true, "-#", -1000},
		{"huge advantage keeps its number but is capped for comparison", core.Eval{Centipawns: 2450}, true, "+24.50", 1000},
	}
	for _, c := range cases {
		got := toEvalDTO(c.eval, c.yourMove)
		if got.Text != c.text || got.Centipawns != c.centipawns {
			t.Errorf("%s: got %q / %d, want %q / %d", c.name, got.Text, got.Centipawns, c.text, c.centipawns)
		}
	}
}

func TestEvalDTOCarriesTheSearchDepth(t *testing.T) {
	if got := toEvalDTO(core.Eval{Depth: 18}, true); got.Depth != 18 {
		t.Errorf("depth = %d, want 18", got.Depth)
	}
}
