package engine

import (
	"context"
	"sync"

	"opposition/internal/core"
)

// Cache remembers evaluations by position so that each position is searched
// once. It lives in memory and is lost when the program exits.
type Cache struct {
	inner core.Evaluator
	mu    sync.Mutex
	evals map[string]core.Eval
}

// NewCache wraps an evaluator with a cache.
func NewCache(inner core.Evaluator) *Cache {
	return &Cache{inner: inner, evals: map[string]core.Eval{}}
}

// Evaluate returns the remembered evaluation of a position or asks the
// wrapped evaluator. Failures are not remembered.
func (c *Cache) Evaluate(ctx context.Context, position string) (core.Eval, error) {
	c.mu.Lock()
	eval, ok := c.evals[position]
	c.mu.Unlock()
	if ok {
		return eval, nil
	}

	eval, err := c.inner.Evaluate(ctx, position)
	if err != nil {
		return core.Eval{}, err
	}
	c.mu.Lock()
	c.evals[position] = eval
	c.mu.Unlock()
	return eval, nil
}
