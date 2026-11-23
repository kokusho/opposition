package core_test

import (
	"testing"

	"opposition/internal/core"
)

func TestLossIsHowMuchWorseTheMoveIsThanTheBestForTheMover(t *testing.T) {
	before := core.Eval{Centipawns: 30} // the mover is 0.30 up with best play
	after := core.Eval{Centipawns: 60}  // after the move the other side is 0.60 up

	if got := core.Loss(before, after); got != 90 {
		t.Errorf("loss = %d, want 90", got)
	}
}

func TestLossIsNeverNegative(t *testing.T) {
	before := core.Eval{Centipawns: 10}
	after := core.Eval{Centipawns: -25} // the search liked the move a bit more one ply deeper

	if got := core.Loss(before, after); got != 0 {
		t.Errorf("loss = %d, want 0", got)
	}
}

func TestLossTreatsMatesAndHugeAdvantagesAsTenPawns(t *testing.T) {
	cases := []struct {
		name          string
		before, after core.Eval
		want          int
	}{
		{"walking into mate from a level game", core.Eval{Centipawns: 0}, core.Eval{IsMate: true, MateIn: 2}, 1000},
		{"keeping a forced mate", core.Eval{IsMate: true, MateIn: 3}, core.Eval{IsMate: true, MateIn: -2}, 0},
		{"delivering mate", core.Eval{IsMate: true, MateIn: 1}, core.Eval{IsMate: true, MateIn: 0}, 0},
		{"still completely winning", core.Eval{Centipawns: 2400}, core.Eval{Centipawns: -1500}, 0},
		{"throwing away a won game", core.Eval{Centipawns: 2400}, core.Eval{Centipawns: 0}, 1000},
	}
	for _, c := range cases {
		if got := core.Loss(c.before, c.after); got != c.want {
			t.Errorf("%s: loss = %d, want %d", c.name, got, c.want)
		}
	}
}
