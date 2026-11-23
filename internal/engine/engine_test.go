package engine_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"opposition/internal/core"
	"opposition/internal/engine"
)

const fake = "testdata/fake-uci.sh"

// startFake runs the fake engine and returns it with a function that reads
// the commands it has received so far.
func startFake(t *testing.T, opts engine.Options) (*engine.Engine, func() string) {
	t.Helper()
	log := filepath.Join(t.TempDir(), "uci.log")
	t.Setenv("FAKE_UCI_LOG", log)
	eng, err := engine.Start(fake, opts)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	t.Cleanup(func() { _ = eng.Close() })
	return eng, func() string {
		data, _ := os.ReadFile(log)
		return string(data)
	}
}

func deadline(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	t.Cleanup(cancel)
	return ctx
}

func TestEvaluateAsksForThePositionAtTheConfiguredDepthAndReturnsTheLastExactScore(t *testing.T) {
	eng, received := startFake(t, engine.Options{Depth: 14})

	eval, err := eng.Evaluate(deadline(t), core.StartPosition)

	if err != nil {
		t.Fatal(err)
	}
	if eval.Centipawns != 42 || eval.Best != "e2e4" || eval.Depth != 2 || len(eval.Line) != 2 {
		t.Errorf("eval = %+v, want 42 centipawns, best e2e4, depth 2, two-move line", eval)
	}
	log := received()
	for _, want := range []string{
		"position fen rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1\n",
		"go depth 14\n",
	} {
		if !strings.Contains(log, want) {
			t.Errorf("engine did not receive %q; got:\n%s", want, log)
		}
	}
}

func TestStartHandshakesAndSetsThreadsAndHash(t *testing.T) {
	_, received := startFake(t, engine.Options{Depth: 10, Threads: 2, HashMB: 64})

	log := received()
	for _, want := range []string{"uci\n", "setoption name Threads value 2\n", "setoption name Hash value 64\n", "isready\n"} {
		if !strings.Contains(log, want) {
			t.Errorf("engine did not receive %q; got:\n%s", want, log)
		}
	}
}

func TestStartFailsWhenTheEngineCannotBeRun(t *testing.T) {
	if _, err := engine.Start("testdata/no-such-engine", engine.Options{Depth: 10}); err == nil {
		t.Error("want an error for a missing engine binary")
	}
}

func TestEvaluateFailsWhenTheEngineDies(t *testing.T) {
	t.Setenv("FAKE_UCI_DIE", "1")
	eng, _ := startFake(t, engine.Options{Depth: 10})

	if _, err := eng.Evaluate(deadline(t), core.StartPosition); err == nil {
		t.Error("want an error when the engine exits mid-search")
	}
}

func TestCloseTellsTheEngineToQuit(t *testing.T) {
	eng, received := startFake(t, engine.Options{Depth: 10})

	if err := eng.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if log := received(); !strings.HasSuffix(log, "quit\n") {
		t.Errorf("last command was not quit; got:\n%s", log)
	}
}

func TestStockfishFindsAMateInOne(t *testing.T) {
	path, err := exec.LookPath("stockfish")
	if err != nil {
		t.Skip("stockfish is not installed")
	}
	eng, err := engine.Start(path, engine.Options{Depth: 10})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	// White to move mates with Ra8.
	eval, err := eng.Evaluate(deadline(t), "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - -")

	if err != nil {
		t.Fatal(err)
	}
	if !eval.IsMate || eval.MateIn != 1 || eval.Best != "a1a8" {
		t.Errorf("eval = %+v, want mate in 1 with a1a8", eval)
	}
}

func TestStockfishStopsWhenCancelledAndStaysUsable(t *testing.T) {
	path, err := exec.LookPath("stockfish")
	if err != nil {
		t.Skip("stockfish is not installed")
	}
	eng, err := engine.Start(path, engine.Options{Depth: 60})
	if err != nil {
		t.Fatal(err)
	}
	defer eng.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = eng.Evaluate(ctx, core.StartPosition)

	if err == nil || time.Since(started) > 5*time.Second {
		t.Fatalf("err = %v after %s, want a prompt context error", err, time.Since(started))
	}

	// The output of the aborted search must not leak into the next one.
	eval, err := eng.Evaluate(deadline(t), "6k1/5ppp/8/8/8/8/5PPP/R5K1 w - -")
	if err != nil || !eval.IsMate || eval.Best != "a1a8" {
		t.Errorf("after cancelling: eval = %+v, err = %v; want the mate in 1", eval, err)
	}
}
