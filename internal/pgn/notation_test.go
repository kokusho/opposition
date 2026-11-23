package pgn_test

import (
	"strings"
	"testing"

	"opposition/internal/core"
	"opposition/internal/pgn"
)

func TestSANLineTurnsEngineMovesIntoReadableNotation(t *testing.T) {
	got, err := pgn.SANLine(core.StartPosition, []string{"e2e4", "e7e5", "g1f3", "b8c6", "f1b5", "a7a6", "b5c6", "d7c6", "e1g1"})

	want := "e4 e5 Nf3 Nc6 Bb5 a6 Bxc6 dxc6 O-O"
	if err != nil || strings.Join(got, " ") != want {
		t.Errorf("got %q (err %v), want %q", strings.Join(got, " "), err, want)
	}
}

func TestSANLineStartsFromAnyPositionKey(t *testing.T) {
	const afterE4 = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -"

	got, err := pgn.SANLine(afterE4, []string{"c7c5"})

	if err != nil || len(got) != 1 || got[0] != "c5" {
		t.Errorf("got %v (err %v), want [c5]", got, err)
	}
}

func TestSANLineRejectsAMoveThatIsNotLegal(t *testing.T) {
	if got, err := pgn.SANLine(core.StartPosition, []string{"e2e4", "e2e4"}); err == nil {
		t.Errorf("got %v, want an error for the second e2e4", got)
	}
}

func TestSANLineRejectsAnUnreadablePosition(t *testing.T) {
	if _, err := pgn.SANLine("not a position", []string{"e2e4"}); err == nil {
		t.Error("want an error for an unreadable position key")
	}
}
