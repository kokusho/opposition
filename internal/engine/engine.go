// Package engine runs a UCI chess engine such as Stockfish as a subprocess.
package engine

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"opposition/internal/core"
)

const (
	defaultDepth     = 16
	handshakeTimeout = 10 * time.Second
	quitTimeout      = 2 * time.Second
)

// Options configures the engine. Zero values leave the engine's defaults,
// except Depth, which falls back to 16.
type Options struct {
	// Depth is how many half-moves deep every position is searched.
	Depth   int
	Threads int
	HashMB  int
}

// Engine is a running UCI engine. It evaluates one position at a time.
type Engine struct {
	mu     sync.Mutex
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	lines  chan string // engine output, closed when the engine exits
	depth  int
	closed bool
}

// Start launches the engine binary and waits until it is ready.
func Start(path string, opts Options) (*Engine, error) {
	cmd := exec.Command(path)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting engine %q: %w", path, err)
	}

	e := &Engine{cmd: cmd, stdin: stdin, lines: make(chan string, 64), depth: opts.Depth}
	if e.depth <= 0 {
		e.depth = defaultDepth
	}
	go func() {
		defer close(e.lines)
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			e.lines <- scanner.Text()
		}
	}()

	if err := e.handshake(opts); err != nil {
		_ = e.Close()
		return nil, fmt.Errorf("engine %q: %w", path, err)
	}
	return e, nil
}

func (e *Engine) handshake(opts Options) error {
	ctx, cancel := context.WithTimeout(context.Background(), handshakeTimeout)
	defer cancel()

	if err := e.send("uci"); err != nil {
		return err
	}
	if err := e.waitFor(ctx, "uciok"); err != nil {
		return err
	}
	if opts.Threads > 0 {
		if err := e.send(fmt.Sprintf("setoption name Threads value %d", opts.Threads)); err != nil {
			return err
		}
	}
	if opts.HashMB > 0 {
		if err := e.send(fmt.Sprintf("setoption name Hash value %d", opts.HashMB)); err != nil {
			return err
		}
	}
	if err := e.send("isready"); err != nil {
		return err
	}
	return e.waitFor(ctx, "readyok")
}

// Evaluate searches the position with the given key to the configured depth.
// The result is from the point of view of the side to move.
func (e *Engine) Evaluate(ctx context.Context, position string) (core.Eval, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return core.Eval{}, errors.New("engine is closed")
	}

	// A position key is a FEN without its two move counters; the search does
	// not depend on them.
	if err := e.send("position fen " + position + " 0 1"); err != nil {
		return core.Eval{}, err
	}
	if err := e.send(fmt.Sprintf("go depth %d", e.depth)); err != nil {
		return core.Eval{}, err
	}

	var eval core.Eval
	scored := false
	cancelled := ctx.Done()
	for {
		select {
		case line, ok := <-e.lines:
			if !ok {
				return core.Eval{}, errors.New("engine exited during the search")
			}
			if parsed, ok := parseInfo(line); ok {
				eval, scored = parsed, true
				continue
			}
			if !strings.HasPrefix(line, "bestmove") {
				continue
			}
			if err := ctx.Err(); err != nil {
				return core.Eval{}, err
			}
			if !scored {
				return core.Eval{}, fmt.Errorf("engine gave no evaluation for %q", position)
			}
			if eval.Best == "" {
				eval.Best = bestMove(line)
			}
			return eval, nil
		case <-cancelled:
			// Ask the engine to stop; it answers with "bestmove", which ends
			// the loop above and keeps the output in step for the next call.
			cancelled = nil
			if err := e.send("stop"); err != nil {
				return core.Eval{}, err
			}
		}
	}
}

// Close asks the engine to quit and waits for it, killing it if it lingers.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.closed {
		return nil
	}
	e.closed = true

	_ = e.send("quit")
	_ = e.stdin.Close()
	exited := make(chan error, 1)
	go func() { exited <- e.cmd.Wait() }()
	select {
	case <-exited:
		return nil
	case <-time.After(quitTimeout):
		_ = e.cmd.Process.Kill()
		<-exited
		return errors.New("engine did not quit and was killed")
	}
}

func (e *Engine) send(command string) error {
	_, err := io.WriteString(e.stdin, command+"\n")
	return err
}

func (e *Engine) waitFor(ctx context.Context, answer string) error {
	for {
		select {
		case line, ok := <-e.lines:
			if !ok {
				return fmt.Errorf("exited before answering %q", answer)
			}
			if strings.TrimSpace(line) == answer {
				return nil
			}
		case <-ctx.Done():
			return fmt.Errorf("no %q within %s: is this a UCI engine?", answer, handshakeTimeout)
		}
	}
}

// bestMove reads the move from a "bestmove e2e4 ponder e7e5" line. It is
// empty when the engine has no move, as in "bestmove (none)".
func bestMove(line string) string {
	fields := strings.Fields(line)
	if len(fields) < 2 || fields[1] == "(none)" {
		return ""
	}
	return fields[1]
}
