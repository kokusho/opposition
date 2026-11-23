package pgn

import (
	"strings"
	"testing"
)

func split(t *testing.T, src string) []string {
	t.Helper()
	var games []string
	for raw, err := range splitGames(strings.NewReader(src)) {
		if err != nil {
			t.Fatalf("splitGames: %v", err)
		}
		games = append(games, raw)
	}
	return games
}

func TestSplitGamesStartsANewGameAtEachTagSection(t *testing.T) {
	src := "[White \"A\"]\n[Black \"B\"]\n\n1. e4 e5 1-0\n\n[White \"C\"]\n[Black \"D\"]\n\n1. d4 d5\n2. c4 0-1\n"

	games := split(t, src)

	if len(games) != 2 {
		t.Fatalf("got %d games, want 2: %q", len(games), games)
	}
	if !strings.HasPrefix(games[0], "[White \"A\"]") || !strings.Contains(games[0], "1. e4 e5 1-0") {
		t.Errorf("first game = %q", games[0])
	}
	if !strings.HasPrefix(games[1], "[White \"C\"]") || !strings.Contains(games[1], "2. c4 0-1") {
		t.Errorf("second game = %q", games[1])
	}
}

func TestSplitGamesIgnoresBracketsInsideAComment(t *testing.T) {
	src := "[White \"A\"]\n\n1. e4 { a long note\n[%clk 1:30:00] still the note } e5 1-0\n\n[White \"C\"]\n\n1. d4 0-1\n"

	games := split(t, src)

	if len(games) != 2 {
		t.Fatalf("got %d games, want 2: %q", len(games), games)
	}
	if !strings.Contains(games[0], "still the note } e5 1-0") {
		t.Errorf("first game lost the rest of its moves: %q", games[0])
	}
}
