package core_test

import (
	"context"
	"errors"
	"testing"

	"opposition/internal/core"
)

// fakeRules lists the legal moves of the positions a test cares about.
type fakeRules map[string][]core.Step

func (f fakeRules) Moves(position string) ([]core.Step, error) {
	return f[position], nil
}

// fakePredictions answers from a table of position -> predicted replies.
type fakePredictions map[string][]core.PredictedMove

func (f fakePredictions) Predict(_ context.Context, position string, _, _ int) (core.Prediction, error) {
	return core.Prediction{Moves: f[position]}, nil
}

func reply(san, after string, probability float64) core.PredictedMove {
	return core.PredictedMove{SAN: san, UCI: "uci-" + san, After: after, Probability: probability}
}

var trapDefaults = core.TrapOptions{
	TheirElo: 1800, MyElo: 1800,
	MinLoss: 100, MinChance: 0.4, MaxOwnLoss: 50, MaxReplies: 5, MinReplyChance: 0.05,
}

// oneTurn is a position where I move and the database knows one move, "push",
// leading to "after push b" where the opponent replies.
func oneTurn() []core.Turn {
	return []core.Turn{{
		Position: "turn w",
		Games:    4,
		Moves:    []core.MoveStat{{SAN: "push", UCI: "uci-push", After: "after push b", Games: 3}},
	}}
}

func TestAMoveAfterWhichTheLikelyReplyLosesIsATrap(t *testing.T) {
	engine := fakeEngine{
		"turn w":       {Centipawns: 20, Best: "uci-push"},
		"after push b": {Centipawns: -20}, // the opponent to move is 0.20 down
		"blundered w":  {Centipawns: 200}, // after the blunder I am 2.00 up
		"held w":       {Centipawns: 20},
	}
	predictions := fakePredictions{
		"after push b": {reply("blunder", "blundered w", 0.6), reply("hold", "held w", 0.4)},
	}

	traps, err := core.FindTraps(context.Background(), oneTurn(), fakeRules{}, engine, predictions, trapDefaults, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(traps) != 1 {
		t.Fatalf("got %d traps, want 1", len(traps))
	}
	trap := traps[0]
	if trap.Move.SAN != "push" || trap.Position != "turn w" || trap.Games != 4 || trap.FacedGames != 3 {
		t.Errorf("trap = %+v, want push at \"turn w\" (4 games, faced 3 times)", trap)
	}
	if !approx(trap.ErrorChance, 0.6) || !approx(trap.ExpectedLoss, 0.6*180) {
		t.Errorf("error chance %v, expected loss %v; want 0.6 and 108", trap.ErrorChance, trap.ExpectedLoss)
	}
	if len(trap.Replies) != 2 || trap.Replies[0].SAN != "blunder" || trap.Replies[0].Loss != 180 || trap.Replies[1].Loss != 0 {
		t.Errorf("replies = %+v, want blunder losing 180 and hold losing 0", trap.Replies)
	}
}

func TestAMoveIsNoTrapWhenGoingWrongIsUnlikely(t *testing.T) {
	engine := fakeEngine{
		"turn w": {Centipawns: 20}, "after push b": {Centipawns: -20},
		"blundered w": {Centipawns: 200}, "held w": {Centipawns: 20},
	}
	predictions := fakePredictions{
		"after push b": {reply("hold", "held w", 0.7), reply("blunder", "blundered w", 0.3)},
	}

	traps, err := core.FindTraps(context.Background(), oneTurn(), fakeRules{}, engine, predictions, trapDefaults, nil)

	if err != nil || len(traps) != 0 {
		t.Errorf("got %+v (err %v), want no trap at a 30%% error chance", traps, err)
	}
}

func TestTheEnginesBestMoveIsTriedEvenIfTheDatabaseNeverSawIt(t *testing.T) {
	turns := []core.Turn{{Position: "turn w", Games: 2}}
	rules := fakeRules{"turn w": {{SAN: "novelty", UCI: "uci-novelty", Position: "after novelty b"}}}
	engine := fakeEngine{
		"turn w":          {Centipawns: 30, Best: "uci-novelty"},
		"after novelty b": {Centipawns: -30},
		"blundered w":     {Centipawns: 250},
	}
	predictions := fakePredictions{"after novelty b": {reply("blunder", "blundered w", 0.9)}}

	traps, err := core.FindTraps(context.Background(), turns, rules, engine, predictions, trapDefaults, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(traps) != 1 || traps[0].Move.SAN != "novelty" || traps[0].FacedGames != 0 {
		t.Errorf("got %+v, want the novelty with 0 games faced", traps)
	}
}

func TestAMoveThatCostsMeTooMuchIsNotOffered(t *testing.T) {
	engine := fakeEngine{
		"turn w":       {Centipawns: 60, Best: "uci-better"},
		"after push b": {Centipawns: 40}, // after my move the opponent is 0.40 up: I gave away a pawn
		"blundered w":  {Centipawns: 300},
	}
	predictions := fakePredictions{"after push b": {reply("blunder", "blundered w", 0.9)}}

	traps, err := core.FindTraps(context.Background(), oneTurn(), fakeRules{}, engine, predictions, trapDefaults, nil)

	if err != nil || len(traps) != 0 {
		t.Errorf("got %+v (err %v), want nothing: the move itself loses 100", traps, err)
	}
}

func TestOnlyTheLikelyRepliesAreSentToTheEngine(t *testing.T) {
	// "rare w" and "sixth w" have no evaluation: asking the engine about them
	// would fail the scan.
	engine := fakeEngine{
		"turn w": {Centipawns: 20}, "after push b": {Centipawns: -20},
		"a w": {Centipawns: 200}, "b w": {Centipawns: 200}, "c w": {Centipawns: 200},
	}
	predictions := fakePredictions{"after push b": {
		reply("a", "a w", 0.5), reply("b", "b w", 0.3), reply("c", "c w", 0.17), reply("rare", "rare w", 0.03),
	}}
	fewer := trapDefaults
	fewer.MaxReplies = 2

	all, err := core.FindTraps(context.Background(), oneTurn(), fakeRules{}, engine, predictions, trapDefaults, nil)
	if err != nil || len(all) != 1 || len(all[0].Replies) != 3 {
		t.Fatalf("got %+v (err %v), want one trap with the three likely replies", all, err)
	}
	limited, err := core.FindTraps(context.Background(), oneTurn(), fakeRules{}, engine, predictions, fewer, nil)
	if err != nil || len(limited) != 1 || len(limited[0].Replies) != 2 {
		t.Errorf("got %+v (err %v), want one trap with two replies", limited, err)
	}
}

func TestTrapsAreOrderedByExpectedLossAndAPositionIsCheckedOnce(t *testing.T) {
	turns := []core.Turn{
		{Position: "first w", Games: 2, Moves: []core.MoveStat{{SAN: "small", UCI: "uci-small", After: "small b", Games: 1}}},
		{Position: "second w", Games: 2, Moves: []core.MoveStat{{SAN: "big", UCI: "uci-big", After: "big b", Games: 1}}},
		// A different route into a position already covered above.
		{Position: "third w", Games: 2, Moves: []core.MoveStat{{SAN: "again", UCI: "uci-again", After: "big b", Games: 1}}},
	}
	engine := fakeEngine{
		"first w": {}, "second w": {}, "third w": {}, "small b": {}, "big b": {},
		"small lost w": {Centipawns: 120}, "big lost w": {Centipawns: 400},
	}
	predictions := fakePredictions{
		"small b": {reply("x", "small lost w", 0.8)},
		"big b":   {reply("y", "big lost w", 0.8)},
	}

	traps, err := core.FindTraps(context.Background(), turns, fakeRules{}, engine, predictions, trapDefaults, nil)

	if err != nil {
		t.Fatal(err)
	}
	if len(traps) != 2 || traps[0].Move.SAN != "big" || traps[1].Move.SAN != "small" {
		t.Errorf("got %+v, want big then small, each once", traps)
	}
}

func TestFindTrapsReportsProgressPerTurnAndStopsOnErrorOrCancel(t *testing.T) {
	engine := fakeEngine{"turn w": {}, "after push b": {}}
	var calls [][2]int

	_, err := core.FindTraps(context.Background(), oneTurn(), fakeRules{}, engine, fakePredictions{}, trapDefaults,
		func(done, total int) { calls = append(calls, [2]int{done, total}) })
	if err != nil || len(calls) != 1 || calls[0] != [2]int{1, 1} {
		t.Errorf("progress = %v (err %v), want [[1 1]]", calls, err)
	}

	if _, err := core.FindTraps(context.Background(), oneTurn(), fakeRules{}, fakeEngine{}, fakePredictions{}, trapDefaults, nil); err == nil {
		t.Error("want the engine's error")
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := core.FindTraps(ctx, oneTurn(), fakeRules{}, engine, fakePredictions{}, trapDefaults, nil); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}
