package engine_test

import (
	"context"
	"errors"
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
