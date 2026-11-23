package engine

import (
	"reflect"
	"testing"

	"opposition/internal/core"
)

func TestParseInfoReadsACentipawnScoreWithItsLine(t *testing.T) {
	line := "info depth 12 seldepth 15 multipv 1 score cp -31 nodes 22841 nps 543833 hashfull 5 tbhits 0 time 42 pv c7c5 g1f3 b8c6"

	got, ok := parseInfo(line)

	want := core.Eval{Centipawns: -31, Depth: 12, Best: "c7c5", Line: []string{"c7c5", "g1f3", "b8c6"}}
	if !ok || !reflect.DeepEqual(got, want) {
		t.Errorf("got %+v (ok=%v), want %+v", got, ok, want)
	}
}

func TestParseInfoReadsAForcedMate(t *testing.T) {
	got, ok := parseInfo("info depth 30 seldepth 2 multipv 1 score mate -3 nodes 90 pv g8h8 a1a8")

	if !ok || !got.IsMate || got.MateIn != -3 || got.Best != "g8h8" {
		t.Errorf("got %+v (ok=%v), want mate in -3 with g8h8", got, ok)
	}
}

func TestParseInfoReadsAPositionThatIsAlreadyMate(t *testing.T) {
	got, ok := parseInfo("info depth 0 score mate 0")

	if !ok || !got.IsMate || got.MateIn != 0 || got.Best != "" || len(got.Line) != 0 {
		t.Errorf("got %+v (ok=%v), want mate in 0 without a move", got, ok)
	}
}

func TestParseInfoSkipsLinesThatAreNotAFinalScore(t *testing.T) {
	for _, line := range []string{
		"info depth 9 seldepth 12 multipv 1 score cp 30 lowerbound nodes 100 pv e2e4",
		"info depth 9 seldepth 12 multipv 1 score cp 30 upperbound nodes 100 pv e2e4",
		"info depth 5 currmove e2e4 currmovenumber 1",
		"info string NNUE evaluation using nn-abc.nnue",
		"bestmove e2e4 ponder e7e5",
		"",
	} {
		if got, ok := parseInfo(line); ok {
			t.Errorf("parseInfo(%q) = %+v, want it skipped", line, got)
		}
	}
}
