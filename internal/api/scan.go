package api

import (
	"context"
	"fmt"
	"net/http"
	"sync"

	"opposition/internal/core"
	"opposition/internal/pgn"
)

const (
	defaultMinGames = 2
	defaultMinLoss  = 50
	defaultMaxPlies = 30
	// refutationMoves is how much of the engine's line is shown.
	refutationMoves = 6
)

type stepDTO struct {
	SAN      string `json:"san"`
	UCI      string `json:"uci"`
	Position string `json:"position"`
}

// mistakeDTO is a habitual move of the opponent that the engine dislikes.
type mistakeDTO struct {
	// Line leads from the start to Position, where the opponent is to move.
	Line      []stepDTO `json:"line"`
	Position  string    `json:"position"`
	NodeGames int       `json:"nodeGames"`
	Move      moveDTO   `json:"move"`
	// Loss is what the move gives away, in centipawns.
	Loss int `json:"loss"`
	// Best is what the engine would have played instead.
	Best string `json:"best,omitempty"`
	// Eval is the position after the move, from your point of view.
	Eval evalDTO `json:"eval"`
	// Refutation is how the engine suggests you continue.
	Refutation []string `json:"refutation"`
}

type scanDTO struct {
	// Status is "running", "done" or "failed".
	Status string       `json:"status"`
	Done   int          `json:"done"`
	Total  int          `json:"total"`
	Error  string       `json:"error,omitempty"`
	Items  []mistakeDTO `json:"items"`
}

func toMistakeDTO(m core.Mistake) mistakeDTO {
	dto := mistakeDTO{
		Line:       make([]stepDTO, 0, len(m.Line)),
		Position:   m.Position,
		NodeGames:  m.NodeGames,
		Move:       toMoveDTO(m.Move),
		Loss:       m.Loss,
		Best:       sanOf(m.Position, m.Before.Best),
		Eval:       toEvalDTO(m.After, true), // after their move it is your turn
		Refutation: []string{},
	}
	for _, step := range m.Line {
		dto.Line = append(dto.Line, stepDTO{SAN: step.SAN, UCI: step.UCI, Position: step.Position})
	}

	line := m.After.Line
	if len(line) == 0 && m.After.Best != "" {
		line = []string{m.After.Best}
	}
	line = line[:min(len(line), refutationMoves)]
	if sans, err := pgn.SANLine(m.Move.After, line); err == nil {
		dto.Refutation = sans
	}
	return dto
}

// scans keeps the mistake scans of this process: finished ones as a cache,
// and at most one that is still running.
type scans struct {
	mu      sync.Mutex
	byKey   map[string]*scan
	running *scan
}

type scan struct {
	cancel context.CancelFunc

	mu    sync.Mutex
	state scanDTO
}

func newScans() *scans {
	return &scans{byKey: map[string]*scan{}}
}

// get returns the scan for key, starting it with run if there is none. A
// different scan that is still running is cancelled first, so the engine
// works on what was asked for last.
func (s *scans) get(key string, run func(ctx context.Context, sc *scan)) *scan {
	s.mu.Lock()
	defer s.mu.Unlock()

	if existing, ok := s.byKey[key]; ok {
		return existing
	}
	if s.running != nil {
		s.running.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	sc := &scan{cancel: cancel, state: scanDTO{Status: "running", Items: []mistakeDTO{}}}
	s.byKey[key] = sc
	s.running = sc
	go func() {
		run(ctx, sc)
		cancel()

		s.mu.Lock()
		defer s.mu.Unlock()
		if s.running == sc {
			s.running = nil
		}
		if sc.snapshot().Status != "done" {
			delete(s.byKey, key) // let the next request try again
		}
	}()
	return sc
}

func (sc *scan) update(change func(state *scanDTO)) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	change(&sc.state)
}

func (sc *scan) snapshot() scanDTO {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	return sc.state
}

func (h *handler) mistakes(w http.ResponseWriter, r *http.Request) {
	if !h.requireEngine(w) {
		return
	}
	sel, ok := h.selection(w, r)
	if !ok {
		return
	}
	minGames, ok := positiveInt(w, r, "minGames", defaultMinGames)
	if !ok {
		return
	}
	minLoss, ok := positiveInt(w, r, "minLoss", defaultMinLoss)
	if !ok {
		return
	}
	maxPlies, ok := positiveInt(w, r, "maxPlies", defaultMaxPlies)
	if !ok {
		return
	}

	key := fmt.Sprintf("%s|%d|%g|%d|%d|%d", sel.opponent, sel.myColor, sel.halfLifeYears, minGames, minLoss, maxPlies)
	sc := h.scans.get(key, func(ctx context.Context, sc *scan) {
		opts := core.ScanOptions{MinGames: minGames, MaxPlies: maxPlies}
		candidates := h.treeFor(sel).Candidates(sel.myColor.Opposite(), opts)
		sc.update(func(state *scanDTO) { state.Total = len(candidates) })

		found, err := core.FindMistakes(ctx, candidates, h.engine, minLoss, func(done, _ int) {
			sc.update(func(state *scanDTO) { state.Done = done })
		})
		sc.update(func(state *scanDTO) {
			if err != nil {
				state.Status, state.Error = "failed", err.Error()
				return
			}
			for _, mistake := range found {
				state.Items = append(state.Items, toMistakeDTO(mistake))
			}
			state.Status = "done"
		})
	})
	writeJSON(w, http.StatusOK, sc.snapshot())
}
