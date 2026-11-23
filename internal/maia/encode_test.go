package maia

import (
	"reflect"
	"testing"

	"opposition/internal/core"
)

// setBits lists the indices of the ones in an encoded board.
func setBits(tokens []float32) []int {
	var bits []int
	for i, value := range tokens {
		if value == 1 {
			bits = append(bits, i)
		} else if value != 0 {
			return []int{-1} // only zeros and ones are allowed
		}
	}
	return bits
}

func TestEncodePutsTheMoversPiecesInTheFirstSixChannels(t *testing.T) {
	tokens, err := encode(core.StartPosition)
	if err != nil {
		t.Fatal(err)
	}

	const rook, king, pawn, theirs = 3, 5, 0, 6
	at := func(square, channel int) float32 { return tokens[square*12+channel] }
	if len(tokens) != 64*12 {
		t.Fatalf("got %d values, want %d", len(tokens), 64*12)
	}
	if at(0, rook) != 1 || at(4, king) != 1 || at(8, pawn) != 1 {
		t.Error("White's rook on a1, king on e1 and pawn on a2 are not in the mover's channels")
	}
	if at(56, theirs+rook) != 1 || at(60, theirs+king) != 1 || at(48, theirs+pawn) != 1 {
		t.Error("Black's rook on a8, king on e8 and pawn on a7 are not in the opponent's channels")
	}
	if got := len(setBits(tokens)); got != 32 {
		t.Errorf("%d squares are occupied, want 32", got)
	}
}

func TestEncodeFlipsTheBoardWhenBlackIsToMove(t *testing.T) {
	tokens, err := encode("rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -")
	if err != nil {
		t.Fatal(err)
	}

	const king, pawn, theirs = 5, 0, 6
	at := func(square, channel int) float32 { return tokens[square*12+channel] }
	if at(4, king) != 1 {
		t.Error("Black's king on e8 should appear on e1 in the mover's channels")
	}
	if at(36, theirs+pawn) != 1 {
		t.Error("White's pawn on e4 should appear on e5 in the opponent's channels")
	}
}

func TestEncodeMatchesTheOfficialTokenizer(t *testing.T) {
	for _, c := range loadReference(t) {
		tokens, err := encode(c.position())
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}
		if got := setBits(tokens); !reflect.DeepEqual(got, c.Tokens) {
			t.Errorf("%s: set bits %v, want %v", c.Name, got, c.Tokens)
		}
	}
}

func TestEncodeRejectsAnUnreadablePosition(t *testing.T) {
	if _, err := encode("not a position"); err == nil {
		t.Error("want an error")
	}
}
