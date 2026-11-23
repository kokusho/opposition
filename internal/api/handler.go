// Package api serves the opening tree of an opponent as JSON over HTTP.
package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"
	"time"

	"opposition/internal/core"
	"opposition/internal/pgn"
)

const (
	defaultHalfLifeYears = 2
	defaultPlayerLimit   = 20
	year                 = time.Duration(365.25 * 24 * float64(time.Hour))
)

// Config is everything the API serves from.
type Config struct {
	Games  []core.Game
	Report pgn.Report
	// Now supplies the date that game ages are measured from.
	Now func() time.Time
	// Engine evaluates positions. Nil means no engine: the endpoints that
	// need one answer 503.
	Engine core.Evaluator
	// Predictor guesses human moves. Nil means no model: the predictions
	// endpoint answers 503.
	Predictor core.Predictor
}

type handler struct {
	games     []core.Game
	report    pgn.Report
	now       func() time.Time
	engine    core.Evaluator
	predictor core.Predictor
	scans     *scans
}

// NewHandler serves the API for one loaded database.
func NewHandler(config Config) http.Handler {
	h := &handler{
		games:     config.Games,
		report:    config.Report,
		now:       config.Now,
		engine:    config.Engine,
		predictor: config.Predictor,
		scans:     newScans(),
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/database", h.database)
	mux.HandleFunc("GET /api/v1/players", h.players)
	mux.HandleFunc("GET /api/v1/tree", h.tree)
	mux.HandleFunc("GET /api/v1/moves", h.moves)
	mux.HandleFunc("GET /api/v1/analysis", h.analysis)
	mux.HandleFunc("GET /api/v1/mistakes", h.mistakes)
	mux.HandleFunc("GET /api/v1/predictions", h.predictions)
	return mux
}

func (h *handler) database(w http.ResponseWriter, _ *http.Request) {
	type skipped struct {
		Number int    `json:"number"`
		Reason string `json:"reason"`
	}
	body := struct {
		Games   int       `json:"games"`
		Skipped []skipped `json:"skipped"`
		// Engine says whether evaluations and the mistake scan are available.
		Engine bool `json:"engine"`
		// Predictions says whether human-move predictions are available.
		Predictions bool `json:"predictions"`
	}{
		Games:       h.report.Games,
		Skipped:     make([]skipped, 0, len(h.report.Skipped)),
		Engine:      h.engine != nil,
		Predictions: h.predictor != nil,
	}
	for _, s := range h.report.Skipped {
		body.Skipped = append(body.Skipped, skipped{Number: s.Number, Reason: s.Reason})
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *handler) players(w http.ResponseWriter, r *http.Request) {
	limit, ok := positiveInt(w, r, "limit", defaultPlayerLimit)
	if !ok {
		return
	}

	players := core.Players(h.games, r.URL.Query().Get("query"))
	body := struct {
		Total int         `json:"total"`
		Items []playerDTO `json:"items"`
	}{Total: len(players), Items: make([]playerDTO, 0, min(limit, len(players)))}
	for _, player := range players[:min(limit, len(players))] {
		body.Items = append(body.Items, playerDTO{Name: player.Name, Games: player.Games, Rating: player.Rating})
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *handler) tree(w http.ResponseWriter, r *http.Request) {
	sel, ok := h.selection(w, r)
	if !ok {
		return
	}
	node := h.treeFor(sel).Node(sel.position)
	writeJSON(w, http.StatusOK, toNodeDTO(sel.position, sel.myColor, node))
}

// selection is what every tree-based endpoint is asked about: whose games,
// seen from which side, weighted how, at which position.
type selection struct {
	opponent      string
	myColor       core.Color
	halfLifeYears float64
	position      string
}

// selection reads and checks the common query parameters. On a problem it
// writes the error response and returns false.
func (h *handler) selection(w http.ResponseWriter, r *http.Request) (selection, bool) {
	query := r.URL.Query()
	sel := selection{
		opponent:      query.Get("opponent"),
		halfLifeYears: defaultHalfLifeYears,
		position:      query.Get("position"),
	}

	if sel.opponent == "" {
		badRequest(w, "opponent is required: the player's name as it appears in the database")
		return sel, false
	}

	switch raw := query.Get("myColor"); raw {
	case "white":
		sel.myColor = core.White
	case "black":
		sel.myColor = core.Black
	default:
		badRequest(w, fmt.Sprintf("myColor must be \"white\" or \"black\", got %q", raw))
		return sel, false
	}

	if raw := query.Get("halfLifeYears"); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
			badRequest(w, fmt.Sprintf("halfLifeYears must be a number of years, 0 or more, got %q", raw))
			return sel, false
		}
		sel.halfLifeYears = parsed
	}

	if sel.position == "" {
		sel.position = core.StartPosition
	}

	if !h.knows(sel.opponent) {
		writeProblem(w, http.StatusNotFound, "Player not found",
			fmt.Sprintf("no game in the database has a player named %q", sel.opponent))
		return sel, false
	}
	return sel, true
}

func (h *handler) treeFor(sel selection) *core.Tree {
	opts := core.TreeOptions{Now: h.now(), HalfLife: time.Duration(sel.halfLifeYears * float64(year))}
	return core.BuildTree(h.games, sel.opponent, sel.myColor.Opposite(), opts)
}

// positiveInt reads an optional whole-number query parameter of at least 1.
// On a problem it writes the error response and returns false.
func positiveInt(w http.ResponseWriter, r *http.Request, name string, fallback int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < 1 {
		badRequest(w, fmt.Sprintf("%s must be a positive whole number, got %q", name, raw))
		return 0, false
	}
	return parsed, true
}

func (h *handler) knows(player string) bool {
	for _, game := range h.games {
		if game.White == player || game.Black == player {
			return true
		}
	}
	return false
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func badRequest(w http.ResponseWriter, detail string) {
	writeProblem(w, http.StatusBadRequest, "Invalid request", detail)
}

func writeProblem(w http.ResponseWriter, status int, title, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Title  string `json:"title"`
		Detail string `json:"detail"`
		Status int    `json:"status"`
	}{title, detail, status})
}
