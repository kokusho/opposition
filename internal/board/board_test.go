package board_test

import (
	"sort"
	"testing"

	"opposition/internal/board"
	"opposition/internal/core"
)

const afterE4 = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -"

func find(moves []board.Move, uci string) (board.Move, bool) {
	for _, move := range moves {
		if move.UCI == uci {
			return move, true
		}
	}
	return board.Move{}, false
}

func TestLegalMovesListsEveryMoveWithNotationAndResultingPosition(t *testing.T) {
	moves, err := board.LegalMoves(core.StartPosition)
	if err != nil {
		t.Fatal(err)
	}

	if len(moves) != 20 {
		t.Fatalf("got %d moves from the start position, want 20", len(moves))
	}
	e4, ok := find(moves, "e2e4")
	if !ok || e4.SAN != "e4" || e4.After != afterE4 {
		t.Errorf("e2e4 = %+v (found=%v), want SAN e4 leading to %q", e4, ok, afterE4)
	}
}

func TestLegalMovesAreOrderedByUCISoTheOrderIsStable(t *testing.T) {
	moves, err := board.LegalMoves(core.StartPosition)
	if err != nil {
		t.Fatal(err)
	}

	ordered := sort.SliceIsSorted(moves, func(i, j int) bool { return moves[i].UCI < moves[j].UCI })
	if !ordered {
		t.Errorf("moves are not ordered by UCI: %+v", moves)
	}
}

func TestLegalMovesIncludeEachPromotionPiece(t *testing.T) {
	moves, err := board.LegalMoves("8/P6k/8/8/8/8/6K1/8 w - -")
	if err != nil {
		t.Fatal(err)
	}

	for uci, san := range map[string]string{"a7a8q": "a8=Q", "a7a8n": "a8=N"} {
		if move, ok := find(moves, uci); !ok || move.SAN != san {
			t.Errorf("%s = %+v (found=%v), want SAN %s", uci, move, ok, san)
		}
	}
}

func TestLegalMovesAreEmptyWhenTheGameIsOver(t *testing.T) {
	// Fool's mate: White is checkmated.
	moves, err := board.LegalMoves("rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq -")

	if err != nil || len(moves) != 0 {
		t.Errorf("got %d moves (err %v), want none", len(moves), err)
	}
}

func TestLegalMovesRejectAnUnreadablePosition(t *testing.T) {
	if _, err := board.LegalMoves("not a position"); err == nil {
		t.Error("want an error")
	}
}
