package pgn_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"opposition/internal/core"
	"opposition/internal/pgn"
)

func load(t *testing.T, src string) ([]core.Game, pgn.Report) {
	t.Helper()
	games, report, err := pgn.Load(strings.NewReader(src))
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return games, report
}

func TestLoadReadsPlayersDateResultAndMovesOfAGame(t *testing.T) {
	src := `[Event "City Open"]
[Date "2026.03.14"]
[White "Brandt, Jonas"]
[Black "Petrov, Ivan"]
[Result "1/2-1/2"]

1. e4 c5 2. Nf3 1/2-1/2
`

	games, _ := load(t, src)

	if len(games) != 1 {
		t.Fatalf("got %d games, want 1", len(games))
	}
	game := games[0]
	if game.White != "Brandt, Jonas" || game.Black != "Petrov, Ivan" {
		t.Errorf("players = %q vs %q", game.White, game.Black)
	}
	if want := time.Date(2026, time.March, 14, 0, 0, 0, 0, time.UTC); !game.Date.Equal(want) {
		t.Errorf("date = %v, want %v", game.Date, want)
	}
	if game.Result != core.Draw {
		t.Errorf("result = %v, want draw", game.Result)
	}
	if len(game.Plies) != 3 {
		t.Fatalf("got %d plies, want 3", len(game.Plies))
	}
	want := core.Ply{
		Before: core.StartPosition,
		SAN:    "e4",
		UCI:    "e2e4",
		After:  "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -",
	}
	if game.Plies[0] != want {
		t.Errorf("first ply = %+v, want %+v", game.Plies[0], want)
	}
}

func TestLoadGivesAPositionTheSameKeyWhenItRecursLaterInTheGame(t *testing.T) {
	games, _ := load(t, "[White \"A\"]\n[Black \"B\"]\n[Result \"*\"]\n\n1. Nf3 Nf6 2. Ng1 Ng8 *\n")

	backHome := games[0].Plies[3].After

	if backHome != core.StartPosition {
		t.Errorf("key after returning to the start = %q, want %q", backHome, core.StartPosition)
	}
}

func TestLoadSkipsAGameWithAnIllegalMoveAndReportsIt(t *testing.T) {
	src := `[White "A"]
[Black "B"]
[Result "1-0"]

1. e4 e5 1-0

[White "Broken"]
[Black "B"]
[Result "1-0"]

1. e4 e5 2. Ke3 1-0

[White "C"]
[Black "D"]
[Result "0-1"]

1. d4 d5 0-1
`

	games, report := load(t, src)

	if len(games) != 2 || games[0].White != "A" || games[1].White != "C" {
		t.Fatalf("got %d games, want the two valid ones (A and C)", len(games))
	}
	if report.Games != 2 {
		t.Errorf("report.Games = %d, want 2", report.Games)
	}
	if len(report.Skipped) != 1 || report.Skipped[0].Number != 2 || report.Skipped[0].Reason == "" {
		t.Errorf("report.Skipped = %+v, want game number 2 with a reason", report.Skipped)
	}
}

func TestLoadConvertsLatin1NamesToUTF8(t *testing.T) {
	latin1 := "[White \"M\xfcller, J\xf6rg\"]\n[Black \"B\"]\n[Result \"*\"]\n\n1. e4 *\n"

	games, _ := load(t, latin1)

	if len(games) != 1 || games[0].White != "Müller, Jörg" {
		t.Errorf("got %+v, want one game with White \"Müller, Jörg\"", games)
	}
}

func TestLoadKeepsUTF8NamesAsTheyAre(t *testing.T) {
	games, _ := load(t, "[White \"Müller, Jörg\"]\n[Black \"B\"]\n[Result \"*\"]\n\n1. e4 *\n")

	if len(games) != 1 || games[0].White != "Müller, Jörg" {
		t.Errorf("got %+v, want one game with White \"Müller, Jörg\"", games)
	}
}

func TestLoadReadsAFileThatStartsWithAByteOrderMark(t *testing.T) {
	games, report := load(t, "\xef\xbb\xbf[White \"A\"]\n[Black \"B\"]\n[Result \"*\"]\n\n1. e4 *\n")

	if len(games) != 1 || games[0].White != "A" || len(report.Skipped) != 0 {
		t.Errorf("got games %+v, skipped %+v; want one game with White \"A\"", games, report.Skipped)
	}
}

func TestSampleDatabaseLoadsAndMergesTheKingsIndianTransposition(t *testing.T) {
	file, err := os.Open("../../testdata/sample.pgn")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()

	games, report, err := pgn.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	if report.Games != 12 || len(report.Skipped) != 0 {
		t.Fatalf("report = %+v, want 12 games and nothing skipped", report)
	}

	// 1.d4 Nf6 2.c4 g6 3.Nc3 Bg7 4.e4 d6 5.Nf3 O-O, also reached via 1.Nf3.
	const kingsIndian = "rnbq1rk1/ppp1ppbp/3p1np1/8/2PPP3/2N2N2/PP3PPP/R1BQKB1R w KQ -"
	node := core.BuildTree(games, "Petrov, Ivan", core.Black, core.TreeOptions{}).Node(kingsIndian)

	if node.Games != 2 || len(node.Moves) != 1 || node.Moves[0].SAN != "Be2" {
		t.Errorf("node = %+v, want Be2 from 2 games", node)
	}
}
