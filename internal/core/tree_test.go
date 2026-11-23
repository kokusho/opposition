package core_test

import (
	"math"
	"strings"
	"testing"
	"time"

	"opposition/internal/core"
)

const start = "start"

const year = 365 * 24 * time.Hour

var now = time.Date(2026, 11, 23, 0, 0, 0, 0, time.UTC)

func approx(a, b float64) bool { return math.Abs(a-b) < 1e-9 }

// line builds plies whose position keys are the moves played so far, so two
// games sharing a prefix share positions. Keys are opaque to the tree.
func line(sans ...string) []core.Ply {
	plies := make([]core.Ply, 0, len(sans))
	before := start
	for i, san := range sans {
		after := strings.Join(sans[:i+1], " ")
		plies = append(plies, core.Ply{Before: before, SAN: san, UCI: "uci-" + san, After: after})
		before = after
	}
	return plies
}

func TestNodeListsTheMovePlayedFromAPosition(t *testing.T) {
	games := []core.Game{
		{White: "Me", Black: "Opp", Plies: line("e4", "c5")},
	}

	node := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{}).Node(start)

	if len(node.Moves) != 1 {
		t.Fatalf("got %d moves, want 1", len(node.Moves))
	}
	got := node.Moves[0]
	if got.SAN != "e4" || got.UCI != "uci-e4" || got.After != "e4" || got.Games != 1 {
		t.Errorf("got %+v, want e4 leading to position \"e4\" with 1 game", got)
	}
}

func TestNodeAggregatesAMoveAcrossGamesMostPlayedFirst(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Plies: line("d4")},
		{White: "B", Black: "Opp", Plies: line("e4")},
		{White: "C", Black: "Opp", Plies: line("e4")},
	}

	node := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{}).Node(start)

	if len(node.Moves) != 2 {
		t.Fatalf("got %d moves, want 2: %+v", len(node.Moves), node.Moves)
	}
	if node.Moves[0].SAN != "e4" || node.Moves[0].Games != 2 {
		t.Errorf("first move = %+v, want e4 with 2 games", node.Moves[0])
	}
	if node.Moves[1].SAN != "d4" || node.Moves[1].Games != 1 {
		t.Errorf("second move = %+v, want d4 with 1 game", node.Moves[1])
	}
}

func TestTreeOnlyUsesGamesWhereTheOpponentHasTheGivenColour(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Plies: line("e4")},
		{White: "Opp", Black: "A", Plies: line("d4")},
		{White: "A", Black: "Someone Else", Plies: line("c4")},
	}

	node := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{}).Node(start)

	if len(node.Moves) != 1 || node.Moves[0].SAN != "e4" {
		t.Errorf("got %+v, want only e4 from the game where Opp is Black", node.Moves)
	}
}

func TestMovesFromAPositionReachedByDifferentMoveOrdersAreMerged(t *testing.T) {
	const transposed = "same position"
	games := []core.Game{
		{White: "A", Black: "Opp", Plies: []core.Ply{
			{Before: start, SAN: "d4", UCI: "d2d4", After: "after d4"},
			{Before: "after d4", SAN: "Nf6", UCI: "g8f6", After: transposed},
			{Before: transposed, SAN: "c4", UCI: "c2c4", After: "x"},
		}},
		{White: "B", Black: "Opp", Plies: []core.Ply{
			{Before: start, SAN: "Nf3", UCI: "g1f3", After: "after Nf3"},
			{Before: "after Nf3", SAN: "Nf6", UCI: "g8f6", After: transposed},
			{Before: transposed, SAN: "c4", UCI: "c2c4", After: "x"},
		}},
	}

	node := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{}).Node(transposed)

	if len(node.Moves) != 1 || node.Moves[0].Games != 2 {
		t.Errorf("got %+v, want c4 with 2 games", node.Moves)
	}
}

func TestMoveCountsResultsFromTheOpponentsPointOfView(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Result: core.BlackWon, Plies: line("e4")},
		{White: "B", Black: "Opp", Result: core.BlackWon, Plies: line("e4")},
		{White: "C", Black: "Opp", Result: core.WhiteWon, Plies: line("e4")},
		{White: "D", Black: "Opp", Result: core.Draw, Plies: line("e4")},
		{White: "E", Black: "Opp", Result: core.ResultUnknown, Plies: line("e4")},
	}

	move := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{}).Node(start).Moves[0]

	if move.Wins != 2 || move.Draws != 1 || move.Losses != 1 {
		t.Errorf("got +%d =%d -%d, want +2 =1 -1 for Opp", move.Wins, move.Draws, move.Losses)
	}
	if move.Games != 5 {
		t.Errorf("got %d games, want 5 (the unfinished game still counts as played)", move.Games)
	}
}

func TestAGameLosesHalfItsWeightPerHalfLifeOfAge(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Date: now, Plies: line("e4")},
		{White: "B", Black: "Opp", Date: now.Add(-2 * year), Plies: line("e4")},
		{White: "C", Black: "Opp", Date: now.Add(-4 * year), Plies: line("e4")},
	}
	opts := core.TreeOptions{Now: now, HalfLife: 2 * year}

	move := core.BuildTree(games, "Opp", core.Black, opts).Node(start).Moves[0]

	if want := 1 + 0.5 + 0.25; !approx(move.Weight, want) {
		t.Errorf("weight = %v, want %v", move.Weight, want)
	}
}

func TestWithoutAHalfLifeEveryGameCountsFully(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Date: now, Plies: line("e4")},
		{White: "B", Black: "Opp", Date: now.Add(-30 * year), Plies: line("e4")},
	}

	move := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{Now: now}).Node(start).Moves[0]

	if !approx(move.Weight, 2) {
		t.Errorf("weight = %v, want 2", move.Weight)
	}
}

func TestMovesAreOrderedByWeightNotByRawGameCount(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Date: now.Add(-20 * year), Plies: line("d4")},
		{White: "B", Black: "Opp", Date: now.Add(-20 * year), Plies: line("d4")},
		{White: "C", Black: "Opp", Date: now, Plies: line("e4")},
	}
	opts := core.TreeOptions{Now: now, HalfLife: 2 * year}

	moves := core.BuildTree(games, "Opp", core.Black, opts).Node(start).Moves

	if moves[0].SAN != "e4" {
		t.Errorf("first move = %s, want the recent e4 ahead of two 20-year-old d4 games", moves[0].SAN)
	}
}

func TestAnUndatedGameCountsAsOldAsTheOpponentsOldestDatedGame(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Date: now, Plies: line("e4")},
		{White: "B", Black: "Opp", Date: now.Add(-4 * year), Plies: line("e4")},
		{White: "C", Black: "Opp", Plies: line("d4")},
		{White: "Opp", Black: "D", Date: now.Add(-40 * year), Plies: line("c4")},
	}
	opts := core.TreeOptions{Now: now, HalfLife: 2 * year}

	moves := core.BuildTree(games, "Opp", core.Black, opts).Node(start).Moves

	undated := moves[len(moves)-1]
	if undated.SAN != "d4" || !approx(undated.Weight, 0.25) {
		t.Errorf("got %+v, want d4 weighted 0.25 like the 4-year-old game", undated)
	}
}

func TestWhenNoGameIsDatedEveryGameCountsFully(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Plies: line("e4")},
		{White: "B", Black: "Opp", Plies: line("e4")},
	}
	opts := core.TreeOptions{Now: now, HalfLife: 2 * year}

	move := core.BuildTree(games, "Opp", core.Black, opts).Node(start).Moves[0]

	if !approx(move.Weight, 2) {
		t.Errorf("weight = %v, want 2", move.Weight)
	}
}

func TestMoveShareIsItsPartOfThePositionsTotalWeight(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Date: now, Plies: line("e4")},
		{White: "B", Black: "Opp", Date: now.Add(-2 * year), Plies: line("d4")},
		{White: "C", Black: "Opp", Date: now.Add(-2 * year), Plies: line("c4")},
	}
	opts := core.TreeOptions{Now: now, HalfLife: 2 * year}

	node := core.BuildTree(games, "Opp", core.Black, opts).Node(start)

	if node.Games != 3 {
		t.Errorf("node games = %d, want 3", node.Games)
	}
	if got := node.Moves[0]; got.SAN != "e4" || !approx(got.Share, 0.5) {
		t.Errorf("got %s with share %v, want e4 with 0.5 (weight 1 of 2)", got.SAN, got.Share)
	}
	if got := node.Moves[1]; !approx(got.Share, 0.25) {
		t.Errorf("got %s with share %v, want 0.25", got.SAN, got.Share)
	}
}

func TestMoveReportsWhenItWasLastPlayed(t *testing.T) {
	recent := now.Add(-1 * year)
	games := []core.Game{
		{White: "A", Black: "Opp", Date: now.Add(-5 * year), Plies: line("e4")},
		{White: "B", Black: "Opp", Date: recent, Plies: line("e4")},
		{White: "C", Black: "Opp", Plies: line("e4")},
	}

	move := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{}).Node(start).Moves[0]

	if !move.LastPlayed.Equal(recent) {
		t.Errorf("last played = %v, want %v", move.LastPlayed, recent)
	}
}

func TestAGameThatRepeatsAPositionCountsOnceForItsMove(t *testing.T) {
	const a, b = "position a", "position b"
	shuffle := []core.Ply{
		{Before: a, SAN: "Nf3", UCI: "g1f3", After: b},
		{Before: b, SAN: "Ng1", UCI: "f3g1", After: a},
		{Before: a, SAN: "Nf3", UCI: "g1f3", After: b},
	}
	games := []core.Game{{White: "A", Black: "Opp", Plies: shuffle}}

	node := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{}).Node(a)

	if node.Games != 1 || node.Moves[0].Games != 1 {
		t.Errorf("got node games %d, move games %d; want 1 and 1", node.Games, node.Moves[0].Games)
	}
}

func TestAPositionTheOpponentNeverReachedIsEmpty(t *testing.T) {
	games := []core.Game{{White: "A", Black: "Opp", Plies: line("e4")}}

	node := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{}).Node("never seen")

	if node.Games != 0 || len(node.Moves) != 0 {
		t.Errorf("got %+v, want an empty node", node)
	}
}

func TestMoveScoreIsTheOpponentsPointsPerFinishedGame(t *testing.T) {
	move := core.MoveStat{Games: 5, Wins: 2, Draws: 1, Losses: 1}

	score, ok := move.Score()

	if !ok || !approx(score, 2.5/4) {
		t.Errorf("score = %v (ok=%v), want 0.625 from +2 =1 -1", score, ok)
	}
}

func TestMoveScoreIsUnknownWithoutAFinishedGame(t *testing.T) {
	move := core.MoveStat{Games: 2}

	if _, ok := move.Score(); ok {
		t.Error("score reported as known for a move with no finished games")
	}
}
