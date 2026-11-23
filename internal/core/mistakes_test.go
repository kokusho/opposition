package core_test

import (
	"context"
	"errors"
	"testing"

	"opposition/internal/core"
)

// fakeEngine answers from a table of position -> evaluation.
type fakeEngine map[string]core.Eval

func (f fakeEngine) Evaluate(_ context.Context, position string) (core.Eval, error) {
	eval, ok := f[position]
	if !ok {
		return core.Eval{}, errors.New("no evaluation for " + position)
	}
	return eval, nil
}

func candidate(position, san, after string, share float64) core.Candidate {
	return core.Candidate{
		Position: position,
		Move:     core.MoveStat{SAN: san, UCI: "uci-" + san, After: after, Games: 2, Share: share},
	}
}

func TestAMoveThatGivesAwayAtLeastTheThresholdIsReported(t *testing.T) {
	engine := fakeEngine{
		"before b": {Centipawns: -20, Best: "e7e6"},
		"after w":  {Centipawns: 110, Best: "d1b3", Line: []string{"d1b3", "b7b6"}},
	}
	candidates := []core.Candidate{candidate("before b", "Bg4", "after w", 1)}

	mistakes, err := core.FindMistakes(context.Background(), candidates, engine, 50, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(mistakes) != 1 {
		t.Fatalf("got %d mistakes, want 1", len(mistakes))
	}
	got := mistakes[0]
	if got.Move.SAN != "Bg4" || got.Loss != 90 || got.Before.Best != "e7e6" || got.After.Best != "d1b3" {
		t.Errorf("got %+v, want Bg4 losing 90 with e7e6 preferred and d1b3 as the reply", got)
	}
}

func TestAMoveThatGivesAwayLessThanTheThresholdIsNotReported(t *testing.T) {
	engine := fakeEngine{"before b": {Centipawns: -20}, "after w": {Centipawns: 50}}
	candidates := []core.Candidate{candidate("before b", "e6", "after w", 1)}

	mistakes, err := core.FindMistakes(context.Background(), candidates, engine, 50, nil)

	if err != nil || len(mistakes) != 0 {
		t.Errorf("got %+v (err %v), want nothing for a 30 centipawn loss", mistakes, err)
	}
}

func TestMistakesAreOrderedByHowOftenTimesHowMuch(t *testing.T) {
	engine := fakeEngine{
		"p1 b": {Centipawns: 0}, "rare blunder w": {Centipawns: 300},
		"p2 b": {Centipawns: 0}, "habit w": {Centipawns: 120},
		"p3 b": {Centipawns: 0}, "small habit w": {Centipawns: 60},
	}
	candidates := []core.Candidate{
		candidate("p1 b", "rare blunder", "rare blunder w", 0.2), // 0.2 x 300 = 60
		candidate("p3 b", "small habit", "small habit w", 0.9),   // 0.9 x  60 = 54
		candidate("p2 b", "habit", "habit w", 0.9),               // 0.9 x 120 = 108
	}

	mistakes, err := core.FindMistakes(context.Background(), candidates, engine, 50, nil)

	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, m := range mistakes {
		order = append(order, m.Move.SAN)
	}
	if !equal(order, []string{"habit", "rare blunder", "small habit"}) {
		t.Errorf("order = %v", order)
	}
}

func TestFindMistakesReportsProgressAfterEachCandidate(t *testing.T) {
	engine := fakeEngine{"p1 b": {}, "a w": {}, "p2 b": {}, "b w": {}}
	candidates := []core.Candidate{candidate("p1 b", "a", "a w", 1), candidate("p2 b", "b", "b w", 1)}
	var calls [][2]int

	_, err := core.FindMistakes(context.Background(), candidates, engine, 50, func(done, total int) {
		calls = append(calls, [2]int{done, total})
	})

	if err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != [2]int{1, 2} || calls[1] != [2]int{2, 2} {
		t.Errorf("progress calls = %v, want [[1 2] [2 2]]", calls)
	}
}

func TestFindMistakesStopsWithTheEnginesError(t *testing.T) {
	candidates := []core.Candidate{candidate("unknown b", "a", "a w", 1)}

	_, err := core.FindMistakes(context.Background(), candidates, fakeEngine{}, 50, nil)

	if err == nil {
		t.Error("want the engine's error, got nil")
	}
}

func TestFindMistakesStopsWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	engine := fakeEngine{"p1 b": {}, "a w": {Centipawns: 500}}
	candidates := []core.Candidate{candidate("p1 b", "a", "a w", 1)}

	mistakes, err := core.FindMistakes(ctx, candidates, engine, 50, nil)

	if !errors.Is(err, context.Canceled) || mistakes != nil {
		t.Errorf("got %v, %v; want nil and context.Canceled", mistakes, err)
	}
}
