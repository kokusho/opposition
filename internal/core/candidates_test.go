package core_test

import (
	"strings"
	"testing"

	"opposition/internal/core"
)

// played builds plies from the real start position. Later positions get the
// key "<moves so far> <side to move>", which is all the scan reads from a key.
func played(sans ...string) []core.Ply {
	plies := make([]core.Ply, 0, len(sans))
	before := core.StartPosition
	for i, san := range sans {
		side := "b"
		if i%2 == 1 {
			side = "w"
		}
		after := strings.Join(sans[:i+1], "-") + " " + side
		plies = append(plies, core.Ply{Before: before, SAN: san, UCI: "uci-" + san, After: after})
		before = after
	}
	return plies
}

func vsBlack(lines ...[]core.Ply) *core.Tree {
	games := make([]core.Game, 0, len(lines))
	for _, plies := range lines {
		games = append(games, core.Game{White: "Someone", Black: "Opp", Plies: plies})
	}
	return core.BuildTree(games, "Opp", core.Black, core.TreeOptions{})
}

func sans(steps []core.Step) string {
	names := make([]string, 0, len(steps))
	for _, step := range steps {
		names = append(names, step.SAN)
	}
	return strings.Join(names, " ")
}

// summary renders candidates as "line | move" for compact comparison.
func summary(candidates []core.Candidate) []string {
	out := make([]string, 0, len(candidates))
	for _, c := range candidates {
		out = append(out, sans(c.Line)+" | "+c.Move.SAN)
	}
	return out
}

func equal(a, b []string) bool {
	return strings.Join(a, ";") == strings.Join(b, ";")
}

var twoGames = core.ScanOptions{MinGames: 2, MaxPlies: 40}

func TestTheOpponentsRepeatedMoveIsACandidate(t *testing.T) {
	tree := vsBlack(played("e4", "c5"), played("e4", "c5"))

	candidates := tree.Candidates(core.Black, twoGames)

	if len(candidates) != 1 {
		t.Fatalf("got %v, want one candidate", summary(candidates))
	}
	got := candidates[0]
	if got.Move.SAN != "c5" || got.Move.Games != 2 || got.Position != "e4 b" || got.NodeGames != 2 {
		t.Errorf("candidate = %+v, want c5 (2 games) in position \"e4 b\"", got)
	}
	wantLine := []core.Step{{SAN: "e4", UCI: "uci-e4", Position: "e4 b"}}
	if len(got.Line) != 1 || got.Line[0] != wantLine[0] {
		t.Errorf("line = %+v, want %+v", got.Line, wantLine)
	}
}

func TestAMovePlayedLessOftenThanMinGamesIsNotACandidate(t *testing.T) {
	tree := vsBlack(played("e4", "c5"), played("e4", "c5"), played("e4", "e5"))

	candidates := tree.Candidates(core.Black, twoGames)

	if got := summary(candidates); !equal(got, []string{"e4 | c5"}) {
		t.Errorf("got %v, want only c5", got)
	}
}

func TestCandidatesAreFoundDeeperInTheLineWithTheMovesLeadingThere(t *testing.T) {
	tree := vsBlack(played("e4", "c5", "Nf3", "d6"), played("e4", "c5", "Nf3", "d6"))

	candidates := tree.Candidates(core.Black, twoGames)

	if got := summary(candidates); !equal(got, []string{"e4 | c5", "e4 c5 Nf3 | d6"}) {
		t.Errorf("got %v, want c5 and then d6 after 1.e4 c5 2.Nf3", got)
	}
}

func TestMovesBehindARarelySeenLineAreNotCandidates(t *testing.T) {
	tree := vsBlack(
		played("e4", "c5", "Nf3", "d6", "d4", "cxd4"),
		played("e4", "c5", "Nf3", "d6", "Bb5+", "Bd7"),
		played("e4", "c5", "c3", "d5"),
	)

	candidates := tree.Candidates(core.Black, twoGames)

	// c3 was faced once and d4/Bb5+ once each, so nothing behind them is a habit.
	if got := summary(candidates); !equal(got, []string{"e4 | c5", "e4 c5 Nf3 | d6"}) {
		t.Errorf("got %v, want c5 and d6 only", got)
	}
}

func TestAPositionReachedByTwoMoveOrdersGivesItsCandidateOnce(t *testing.T) {
	const tabiya = "tabiya b"
	viaD4 := []core.Ply{
		{Before: core.StartPosition, SAN: "d4", UCI: "d2d4", After: "d4 b"},
		{Before: "d4 b", SAN: "Nf6", UCI: "g8f6", After: "d4-Nf6 w"},
		{Before: "d4-Nf6 w", SAN: "c4", UCI: "c2c4", After: tabiya},
		{Before: tabiya, SAN: "g6", UCI: "g7g6", After: "after g6 w"},
	}
	viaC4 := []core.Ply{
		{Before: core.StartPosition, SAN: "c4", UCI: "c2c4", After: "c4 b"},
		{Before: "c4 b", SAN: "Nf6", UCI: "g8f6", After: "c4-Nf6 w"},
		{Before: "c4-Nf6 w", SAN: "d4", UCI: "d2d4", After: tabiya},
		{Before: tabiya, SAN: "g6", UCI: "g7g6", After: "after g6 w"},
	}
	tree := vsBlack(viaD4, viaD4, viaC4)

	candidates := tree.Candidates(core.Black, twoGames)

	// 1...Nf6 after 1.d4 was played twice; 1.c4 Nf6 only once. The tabiya has
	// three games and must be listed once, by its most played route.
	if got := summary(candidates); !equal(got, []string{"d4 | Nf6", "d4 Nf6 c4 | g6"}) {
		t.Errorf("got %v", got)
	}
}

func TestTheScanStopsAfterMaxPlies(t *testing.T) {
	tree := vsBlack(played("e4", "c5", "Nf3", "d6"), played("e4", "c5", "Nf3", "d6"))

	candidates := tree.Candidates(core.Black, core.ScanOptions{MinGames: 2, MaxPlies: 2})

	// 1...c5 is the 2nd half-move, 2...d6 the 4th.
	if got := summary(candidates); !equal(got, []string{"e4 | c5"}) {
		t.Errorf("got %v, want only c5", got)
	}
}

func TestWhenTheOpponentIsWhiteTheirFirstMoveIsACandidateWithAnEmptyLine(t *testing.T) {
	games := []core.Game{
		{White: "Opp", Black: "A", Plies: played("d4", "d5")},
		{White: "Opp", Black: "B", Plies: played("d4", "Nf6")},
	}
	tree := core.BuildTree(games, "Opp", core.White, core.TreeOptions{})

	candidates := tree.Candidates(core.White, twoGames)

	if len(candidates) != 1 || candidates[0].Move.SAN != "d4" || len(candidates[0].Line) != 0 ||
		candidates[0].Position != core.StartPosition {
		t.Errorf("got %+v, want d4 from the start position", candidates)
	}
}
