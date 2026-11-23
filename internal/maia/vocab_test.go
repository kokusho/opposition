package maia

import "testing"

func TestMoveIndexNumbersFromAndToSquaresRankByRank(t *testing.T) {
	cases := map[string]int{
		"a1a1": 0,
		"a1b1": 1,
		"a1a2": 8,
		"b1a1": 64,
		"e2e4": 12*64 + 28,
		"e1g1": 4*64 + 6, // castling is the king's two-square move
		"h8h8": 4095,
	}
	for uci, want := range cases {
		if got, ok := moveIndex(uci); !ok || got != want {
			t.Errorf("moveIndex(%q) = %d (ok=%v), want %d", uci, got, ok, want)
		}
	}
}

func TestMoveIndexPutsPromotionsAfterThePlainMoves(t *testing.T) {
	cases := map[string]int{
		"a7a8q": 4096,
		"a7a8r": 4097,
		"a7a8b": 4098,
		"a7a8n": 4099,
		"a7b8q": 4100,
		"e7e8q": 4096 + (4*8+4)*4,
		"e7d8n": 4096 + (4*8+3)*4 + 3,
		"h7h8n": 4351,
	}
	for uci, want := range cases {
		if got, ok := moveIndex(uci); !ok || got != want {
			t.Errorf("moveIndex(%q) = %d (ok=%v), want %d", uci, got, ok, want)
		}
	}
}

func TestMoveIndexRejectsWhatIsNotInTheVocabulary(t *testing.T) {
	for _, uci := range []string{"", "e2", "e2e9", "i2e4", "e7e8k", "e2e1q", "e7e8qq"} {
		if got, ok := moveIndex(uci); ok {
			t.Errorf("moveIndex(%q) = %d, want it rejected", uci, got)
		}
	}
}

func TestMirrorMoveFlipsTheRanksAndKeepsThePromotionPiece(t *testing.T) {
	cases := map[string]string{
		"e7e5":  "e2e4",
		"e8g8":  "e1g1",
		"g8f6":  "g1f3",
		"a2a1q": "a7a8q",
		"b2a1n": "b7a8n",
	}
	for uci, want := range cases {
		if got := mirrorMove(uci); got != want {
			t.Errorf("mirrorMove(%q) = %q, want %q", uci, got, want)
		}
	}
}
