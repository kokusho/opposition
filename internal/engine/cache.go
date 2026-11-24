package engine

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"

	"opposition/internal/core"
)

// Cache remembers evaluations by position so that each position is searched
// once. Opened on a file, it also keeps them across runs of the program.
type Cache struct {
	inner core.Evaluator
	mu    sync.Mutex
	evals map[string]core.Eval
	file  *os.File // nil for a cache that lives in memory only
}

// record is one line of a cache file.
type record struct {
	Position string    `json:"position"`
	Eval     core.Eval `json:"eval"`
}

// NewCache wraps an evaluator with a cache that lives in memory.
func NewCache(inner core.Evaluator) *Cache {
	return &Cache{inner: inner, evals: map[string]core.Eval{}}
}

// OpenCache wraps an evaluator with a cache kept in the file at path, one
// evaluation per line. The file and its directory are created if missing;
// lines that cannot be read, such as one cut off by a crash, are skipped. One
// file should hold evaluations of one engine at one depth.
func OpenCache(inner core.Evaluator, path string) (*Cache, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}

	cache := &Cache{inner: inner, evals: map[string]core.Eval{}, file: file}
	lines := bufio.NewScanner(file)
	lines.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for lines.Scan() {
		var stored record
		if json.Unmarshal(lines.Bytes(), &stored) == nil && stored.Position != "" {
			cache.evals[stored.Position] = stored.Eval
		}
	}
	if err := lines.Err(); err != nil {
		file.Close()
		return nil, err
	}
	return cache, nil
}

// Len is the number of evaluations held.
func (c *Cache) Len() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.evals)
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
	defer c.mu.Unlock()
	c.evals[position] = eval
	if c.file != nil {
		// Losing a line only costs a repeated search, so a failed write is
		// not worth failing the evaluation for.
		if line, err := json.Marshal(record{Position: position, Eval: eval}); err == nil {
			_, _ = c.file.Write(append(line, '\n'))
		}
	}
	return eval, nil
}

// Close releases the cache file, if there is one.
func (c *Cache) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.file == nil {
		return nil
	}
	err := c.file.Close()
	c.file = nil
	return err
}
