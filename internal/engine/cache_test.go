package engine_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"opposition/internal/core"
	"opposition/internal/engine"
)

// countingEngine counts how often each position is evaluated.
type countingEngine struct {
	calls map[string]int
	fail  bool
}

func (c *countingEngine) Evaluate(_ context.Context, position string) (core.Eval, error) {
	c.calls[position]++
	if c.fail {
		return core.Eval{}, errors.New("engine trouble")
	}
	return core.Eval{Centipawns: len(position)}, nil
}

func TestCacheAsksTheEngineOncePerPosition(t *testing.T) {
	inner := &countingEngine{calls: map[string]int{}}
	cache := engine.NewCache(inner)

	first, _ := cache.Evaluate(context.Background(), "a")
	second, _ := cache.Evaluate(context.Background(), "a")
	other, _ := cache.Evaluate(context.Background(), "bb")

	if inner.calls["a"] != 1 || inner.calls["bb"] != 1 {
		t.Errorf("engine calls = %v, want one per position", inner.calls)
	}
	if first.Centipawns != 1 || second.Centipawns != 1 || other.Centipawns != 2 {
		t.Errorf("got %d, %d, %d; want 1, 1, 2", first.Centipawns, second.Centipawns, other.Centipawns)
	}
}

func TestCacheDoesNotRememberFailures(t *testing.T) {
	inner := &countingEngine{calls: map[string]int{}, fail: true}
	cache := engine.NewCache(inner)

	_, firstErr := cache.Evaluate(context.Background(), "a")
	inner.fail = false
	eval, secondErr := cache.Evaluate(context.Background(), "a")

	if firstErr == nil || secondErr != nil || eval.Centipawns != 1 {
		t.Errorf("first err %v, second err %v, eval %+v; want a failure and then a fresh result", firstErr, secondErr, eval)
	}
}

func TestEvaluationsSurviveReopeningTheCacheFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "evals.jsonl")
	first := &countingEngine{calls: map[string]int{}}
	cache, err := engine.OpenCache(first, path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := cache.Evaluate(context.Background(), "abc"); err != nil {
		t.Fatal(err)
	}
	if err := cache.Close(); err != nil {
		t.Fatal(err)
	}

	second := &countingEngine{calls: map[string]int{}}
	reopened, err := engine.OpenCache(second, path)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	eval, err := reopened.Evaluate(context.Background(), "abc")

	if err != nil || eval.Centipawns != 3 || second.calls["abc"] != 0 {
		t.Errorf("eval %+v, err %v, engine calls %v; want the stored evaluation without asking the engine", eval, err, second.calls)
	}
}

func TestADamagedLineInTheCacheFileIsSkipped(t *testing.T) {
	path := filepath.Join(t.TempDir(), "evals.jsonl")
	content := `{"position":"good","eval":{"Centipawns":42}}` + "\n" + `{"position":"cut off` + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	inner := &countingEngine{calls: map[string]int{}}

	cache, err := engine.OpenCache(inner, path)
	if err != nil {
		t.Fatal(err)
	}
	defer cache.Close()
	eval, err := cache.Evaluate(context.Background(), "good")

	if err != nil || eval.Centipawns != 42 || inner.calls["good"] != 0 {
		t.Errorf("eval %+v, err %v, engine calls %v; want 42 from the file", eval, err, inner.calls)
	}
}
