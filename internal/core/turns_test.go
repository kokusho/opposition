package core_test

import (
	"testing"

	"opposition/internal/core"
)

// turnSummary renders turns as "line -> faced moves" for compact comparison.
func turnSummary(turns []core.Turn) []string {
	out := make([]string, 0, len(turns))
	for _, turn := range turns {
		faced := ""
		for i, move := range turn.Moves {
			if i > 0 {
				faced += ","
			}
			faced += move.SAN
		}
		out = append(out, sans(turn.Line)+" -> "+faced)
	}
	return out
}

func TestMyTurnsAreThePositionsWhereIMoveInsideTheirRepertoire(t *testing.T) {
	tree := vsBlack(played("e4", "c5", "Nf3", "d6"), played("e4", "c5", "c3", "d5"))

	turns := tree.MyTurns(core.White, twoGames)

	// Both games reach the start position and the position after 1...c5;
	// what follows was reached once only and is not their repertoire.
	if got := turnSummary(turns); !equal(got, []string{" -> e4", "e4 c5 -> Nf3,c3"}) {
		t.Errorf("got %q", got)
	}
	if turns[1].Games != 2 || turns[1].Position != "e4-c5 w" {
		t.Errorf("second turn = %+v, want 2 games at \"e4-c5 w\"", turns[1])
	}
}

func TestMyTurnsAsBlackStartAfterTheirFirstMove(t *testing.T) {
	games := []core.Game{
		{White: "Opp", Black: "A", Plies: played("d4", "d5")},
		{White: "Opp", Black: "B", Plies: played("d4", "Nf6")},
	}
	tree := core.BuildTree(games, "Opp", core.White, core.TreeOptions{})

	turns := tree.MyTurns(core.Black, twoGames)

	if got := turnSummary(turns); !equal(got, []string{"d4 -> Nf6,d5"}) {
		t.Errorf("got %q", got)
	}
}

func TestMyTurnsStopAfterMaxPliesAndListATranspositionOnce(t *testing.T) {
	const tabiya = "tabiya w"
	viaD4 := []core.Ply{
		{Before: core.StartPosition, SAN: "d4", UCI: "d2d4", After: "d4 b"},
		{Before: "d4 b", SAN: "Nf6", UCI: "g8f6", After: tabiya},
		{Before: tabiya, SAN: "c4", UCI: "c2c4", After: "after c4 b"},
	}
	viaNf3 := []core.Ply{
		{Before: core.StartPosition, SAN: "Nf3", UCI: "g1f3", After: "Nf3 b"},
		{Before: "Nf3 b", SAN: "Nf6", UCI: "g8f6", After: tabiya},
		{Before: tabiya, SAN: "c4", UCI: "c2c4", After: "after c4 b"},
	}
	tree := vsBlack(viaD4, viaD4, viaNf3)

	all := tree.MyTurns(core.White, twoGames)
	shallow := tree.MyTurns(core.White, core.ScanOptions{MinGames: 2, MaxPlies: 2})

	if got := turnSummary(all); !equal(got, []string{" -> d4,Nf3", "d4 Nf6 -> c4"}) {
		t.Errorf("all turns = %q", got)
	}
	if got := turnSummary(shallow); !equal(got, []string{" -> d4,Nf3"}) {
		t.Errorf("turns within two half-moves = %q", got)
	}
}
