package core_test

import (
	"testing"

	"opposition/internal/core"
)

func TestSummaryCountsTheOpponentsGamesAndResultsWithThatColour(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Result: core.BlackWon, Date: day(2024, 5, 1)},
		{White: "B", Black: "Opp", Result: core.WhiteWon, Date: day(2026, 3, 1)},
		{White: "C", Black: "Opp", Result: core.Draw},
		{White: "D", Black: "Opp"},
		{White: "Opp", Black: "E", Result: core.WhiteWon, Date: day(2010, 1, 1)},
	}

	got := core.Summarize(games, "Opp", core.Black)

	if got.Games != 4 || got.Wins != 1 || got.Draws != 1 || got.Losses != 1 {
		t.Errorf("got %d games +%d =%d -%d, want 4 games +1 =1 -1", got.Games, got.Wins, got.Draws, got.Losses)
	}
	if !got.First.Equal(day(2024, 5, 1)) || !got.Last.Equal(day(2026, 3, 1)) {
		t.Errorf("dates %v to %v, want 2024-05-01 to 2026-03-01", got.First, got.Last)
	}
}

// lineSummary renders lines as "moves (games)".
func lineSummary(lines []core.Line) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		out = append(out, sans(line.Steps)+" ("+string(rune('0'+line.Games))+")")
	}
	return out
}

var lineDefaults = core.LineOptions{MinGames: 2, MaxPlies: 20, MaxLines: 8}

func TestMainLinesFollowTheMovesPlayedRepeatedlyMostPlayedFirst(t *testing.T) {
	tree := vsBlack(
		played("e4", "c5", "Nf3", "d6"), played("e4", "c5", "Nf3", "d6"), played("e4", "c5", "Nf3", "d6"),
		played("e4", "c5", "c3", "d5"), played("e4", "c5", "c3", "d5"),
		played("d4", "Nf6"),
	)

	lines := tree.MainLines(lineDefaults)

	if got := lineSummary(lines); !equal(got, []string{"e4 c5 Nf3 d6 (3)", "e4 c5 c3 d5 (2)"}) {
		t.Errorf("got %q", got)
	}
}

func TestAMainLineEndsWhereTheGamesDiverge(t *testing.T) {
	tree := vsBlack(played("e4", "c5", "Nf3", "d6"), played("e4", "c5", "Nf3", "e6"))

	lines := tree.MainLines(lineDefaults)

	if got := lineSummary(lines); !equal(got, []string{"e4 c5 Nf3 (2)"}) {
		t.Errorf("got %q, want the shared part only", got)
	}
}

func TestMainLinesCarryTheOpponentsResultsAtTheirEnd(t *testing.T) {
	games := []core.Game{
		{White: "A", Black: "Opp", Result: core.BlackWon, Plies: played("e4", "c5")},
		{White: "B", Black: "Opp", Result: core.Draw, Plies: played("e4", "c5")},
	}
	tree := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{})

	lines := tree.MainLines(lineDefaults)

	if len(lines) != 1 || lines[0].Wins != 1 || lines[0].Draws != 1 || lines[0].Losses != 0 {
		t.Errorf("got %+v, want one line with +1 =1 -0", lines)
	}
}

func TestMainLinesAreLimitedInLengthAndNumber(t *testing.T) {
	tree := vsBlack(
		played("e4", "c5", "Nf3", "d6"), played("e4", "c5", "Nf3", "d6"), played("e4", "c5", "Nf3", "d6"),
		played("d4", "Nf6", "c4", "g6"), played("d4", "Nf6", "c4", "g6"),
	)

	short := tree.MainLines(core.LineOptions{MinGames: 2, MaxPlies: 2, MaxLines: 8})
	one := tree.MainLines(core.LineOptions{MinGames: 2, MaxPlies: 20, MaxLines: 1})

	if got := lineSummary(short); !equal(got, []string{"e4 c5 (3)", "d4 Nf6 (2)"}) {
		t.Errorf("two half-moves deep: %q", got)
	}
	if got := lineSummary(one); !equal(got, []string{"e4 c5 Nf3 d6 (3)"}) {
		t.Errorf("one line: %q", got)
	}
}

func scored(result core.Result, sans ...string) core.Game {
	return core.Game{White: "Someone", Black: "Opp", Result: result, Plies: played(sans...)}
}

func TestWeakSpotsAreTheMovesTheOpponentScoresBadlyAgainst(t *testing.T) {
	games := []core.Game{
		scored(core.WhiteWon, "e4", "c5", "c3"), scored(core.WhiteWon, "e4", "c5", "c3"), scored(core.Draw, "e4", "c5", "c3"),
		scored(core.BlackWon, "e4", "c5", "Nf3"), scored(core.BlackWon, "e4", "c5", "Nf3"),
		scored(core.WhiteWon, "e4", "c5", "d4"),
	}
	tree := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{})

	spots := tree.WeakSpots(core.White, twoGames, 0.35)

	// 1.e4 itself: Opp scored 2.5/6, above the limit. 2.c3: 0.5/3. 2.d4 was
	// faced once only. 2.Nf3: Opp won both.
	if len(spots) != 1 || spots[0].Move.SAN != "c3" || sans(spots[0].Line) != "e4 c5" || spots[0].Move.Games != 3 {
		t.Errorf("got %+v, want only 2.c3 after 1.e4 c5", spots)
	}
}

func TestWeakSpotsNeedFinishedGames(t *testing.T) {
	games := []core.Game{scored(core.ResultUnknown, "e4"), scored(core.ResultUnknown, "e4")}
	tree := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{})

	if spots := tree.WeakSpots(core.White, twoGames, 0.35); len(spots) != 0 {
		t.Errorf("got %+v, want nothing without results", spots)
	}
}

func TestAMoveIsNotAWeakSpotWhereNothingElseWasFaced(t *testing.T) {
	games := []core.Game{
		scored(core.WhiteWon, "e4", "c5", "c3", "d5", "exd5"), scored(core.WhiteWon, "e4", "c5", "c3", "d5", "exd5"),
		scored(core.BlackWon, "e4", "c5", "Nf3"), scored(core.BlackWon, "e4", "c5", "Nf3"),
	}
	tree := core.BuildTree(games, "Opp", core.Black, core.TreeOptions{})

	spots := tree.WeakSpots(core.White, twoGames, 0.35)

	// 2.c3 is where the lost games part from the won ones. 3.exd5 scores just
	// as badly but only repeats that; 1.e4 was the only first move faced.
	if len(spots) != 1 || spots[0].Move.SAN != "c3" {
		t.Errorf("got %+v, want only 2.c3", spots)
	}
}
