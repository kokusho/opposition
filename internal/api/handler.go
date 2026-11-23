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

type handler struct {
	games  []core.Game
	report pgn.Report
	now    func() time.Time
}

// NewHandler serves the API for one loaded database. now supplies the date
// that game ages are measured from.
func NewHandler(games []core.Game, report pgn.Report, now func() time.Time) http.Handler {
	h := &handler{games: games, report: report, now: now}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/database", h.database)
	mux.HandleFunc("GET /api/v1/players", h.players)
	mux.HandleFunc("GET /api/v1/tree", h.tree)
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
	}{Games: h.report.Games, Skipped: make([]skipped, 0, len(h.report.Skipped))}
	for _, s := range h.report.Skipped {
		body.Skipped = append(body.Skipped, skipped{Number: s.Number, Reason: s.Reason})
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *handler) players(w http.ResponseWriter, r *http.Request) {
	limit := defaultPlayerLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 {
			badRequest(w, fmt.Sprintf("limit must be a positive whole number, got %q", raw))
			return
		}
		limit = parsed
	}

	players := core.Players(h.games, r.URL.Query().Get("query"))
	body := struct {
		Total int         `json:"total"`
		Items []playerDTO `json:"items"`
	}{Total: len(players), Items: make([]playerDTO, 0, min(limit, len(players)))}
	for _, player := range players[:min(limit, len(players))] {
		body.Items = append(body.Items, playerDTO{Name: player.Name, Games: player.Games})
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *handler) tree(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	opponent := query.Get("opponent")
	if opponent == "" {
		badRequest(w, "opponent is required: the player's name as it appears in the database")
		return
	}

	var myColor core.Color
	switch raw := query.Get("myColor"); raw {
	case "white":
		myColor = core.White
	case "black":
		myColor = core.Black
	default:
		badRequest(w, fmt.Sprintf("myColor must be \"white\" or \"black\", got %q", raw))
		return
	}

	halfLifeYears := float64(defaultHalfLifeYears)
	if raw := query.Get("halfLifeYears"); raw != "" {
		parsed, err := strconv.ParseFloat(raw, 64)
		if err != nil || math.IsNaN(parsed) || math.IsInf(parsed, 0) || parsed < 0 {
			badRequest(w, fmt.Sprintf("halfLifeYears must be a number of years, 0 or more, got %q", raw))
			return
		}
		halfLifeYears = parsed
	}

	position := query.Get("position")
	if position == "" {
		position = core.StartPosition
	}

	if !h.knows(opponent) {
		writeProblem(w, http.StatusNotFound, "Player not found",
			fmt.Sprintf("no game in the database has a player named %q", opponent))
		return
	}

	opts := core.TreeOptions{Now: h.now(), HalfLife: time.Duration(halfLifeYears * float64(year))}
	node := core.BuildTree(h.games, opponent, myColor.Opposite(), opts).Node(position)
	writeJSON(w, http.StatusOK, toNodeDTO(position, myColor, node))
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
