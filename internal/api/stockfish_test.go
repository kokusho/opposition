package api_test

import (
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"testing"
	"time"

	"opposition/internal/api"
	"opposition/internal/engine"
	"opposition/internal/pgn"
)

// The whole chain with the real engine: a player who twice answered
// 1.e4 e5 2.Nf3 with 2...f6 gets that move reported.
func TestScanWithStockfishFindsARepeatedOpeningBlunder(t *testing.T) {
	path, err := exec.LookPath("stockfish")
	if err != nil {
		t.Skip("stockfish is not installed")
	}
	file, err := os.Open("../../testdata/habits.pgn")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	games, report, err := pgn.Load(file)
	if err != nil || len(report.Skipped) != 0 {
		t.Fatalf("loading habits.pgn: err %v, skipped %+v", err, report.Skipped)
	}
	stockfish, err := engine.Start(path, engine.Options{Depth: 12})
	if err != nil {
		t.Fatal(err)
	}
	defer stockfish.Close()
	handler := api.NewHandler(api.Config{Games: games, Report: report, Now: time.Now, Engine: engine.NewCache(stockfish)})
	query := url.Values{"opponent": {"Sorglos, Hans"}, "myColor": {"white"}}

	var body mistakesJSON
	for deadline := time.Now().Add(60 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		body = decode[mistakesJSON](t, get(t, handler, "/api/v1/mistakes", query), http.StatusOK)
		if body.Status != "running" {
			break
		}
	}

	if body.Status != "done" {
		t.Fatalf("status %q, error %q", body.Status, body.Error)
	}
	if len(body.Items) != 1 || body.Items[0].Move.SAN != "f6" {
		t.Fatalf("got %+v, want exactly 2...f6", body.Items)
	}
	f6 := body.Items[0]
	// Which reply the engine prefers depends on depth and version, so only
	// its presence is checked.
	if f6.Loss < 100 || len(f6.Refutation) == 0 || f6.Best == "" || f6.Best == "f6" {
		t.Errorf("f6: loss %d, preferred %q, refutation %v; want over a pawn lost, another move preferred, and a reply",
			f6.Loss, f6.Best, f6.Refutation)
	}
}
