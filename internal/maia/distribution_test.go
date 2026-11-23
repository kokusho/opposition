package maia

import (
	"math"
	"testing"

	"opposition/internal/board"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-6 }

func logitsWith(values map[string]float32) []float32 {
	logits := make([]float32, vocabularySize)
	for uci, value := range values {
		index, ok := moveIndex(uci)
		if !ok {
			panic("not in vocabulary: " + uci)
		}
		logits[index] = value
	}
	return logits
}

func TestDistributionSpreadsProbabilityOverTheLegalMovesOnly(t *testing.T) {
	// An illegal move with a huge score must not take any probability.
	logits := logitsWith(map[string]float32{"e2e4": float32(math.Log(3)), "d2d4": 0, "a1h8": 50})
	legal := []board.Move{
		{UCI: "d2d4", SAN: "d4", After: "after d4"},
		{UCI: "e2e4", SAN: "e4", After: "after e4"},
	}

	moves := distribution(logits, legal, false)

	if len(moves) != 2 {
		t.Fatalf("got %d moves, want 2", len(moves))
	}
	if moves[0].UCI != "e2e4" || !near(moves[0].Probability, 0.75) || moves[0].SAN != "e4" || moves[0].After != "after e4" {
		t.Errorf("first = %+v, want e2e4 at 0.75 with its notation and position", moves[0])
	}
	if moves[1].UCI != "d2d4" || !near(moves[1].Probability, 0.25) {
		t.Errorf("second = %+v, want d2d4 at 0.25", moves[1])
	}
}

func TestDistributionLooksUpBlacksMovesMirrored(t *testing.T) {
	// The model sees Black's e7e5 as e2e4.
	logits := logitsWith(map[string]float32{"e2e4": float32(math.Log(4))})
	legal := []board.Move{{UCI: "d7d5"}, {UCI: "e7e5"}}

	moves := distribution(logits, legal, true)

	if moves[0].UCI != "e7e5" || !near(moves[0].Probability, 0.8) {
		t.Errorf("first = %+v, want e7e5 at 0.8", moves[0])
	}
}

func TestDistributionOfNoLegalMovesIsEmpty(t *testing.T) {
	if moves := distribution(make([]float32, vocabularySize), nil, false); len(moves) != 0 {
		t.Errorf("got %+v, want nothing", moves)
	}
}

func TestOutcomeReadsLossDrawWinInThatOrder(t *testing.T) {
	// Logits are [loss, draw, win] for the side to move.
	win, draw, loss := outcome([]float32{0, float32(math.Log(2)), float32(math.Log(5))})

	if !near(loss, 1.0/8) || !near(draw, 2.0/8) || !near(win, 5.0/8) {
		t.Errorf("win %v draw %v loss %v, want 0.625 0.25 0.125", win, draw, loss)
	}
}
