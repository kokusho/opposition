package api

import (
	"fmt"
	"net/http"
	"strconv"

	"opposition/internal/board"
	"opposition/internal/core"
)

// Ratings outside this range are far from anything the model was trained on.
const (
	minElo = 100
	maxElo = 4000
)

type legalMoveDTO struct {
	SAN string `json:"san"`
	UCI string `json:"uci"`
	// Position is the key of the position after the move.
	Position string `json:"position"`
}

type predictedMoveDTO struct {
	SAN      string `json:"san"`
	UCI      string `json:"uci"`
	Position string `json:"position"`
	// Probability that a player of the given rating plays this move, 0 to 1.
	Probability float64 `json:"probability"`
}

type predictionDTO struct {
	Position string `json:"position"`
	// SelfElo is the rating of the side to move, OppoElo of the other player.
	SelfElo int `json:"selfElo"`
	OppoElo int `json:"oppoElo"`
	// Win, Draw and Loss predict the result for the side to move.
	Win   float64            `json:"win"`
	Draw  float64            `json:"draw"`
	Loss  float64            `json:"loss"`
	Moves []predictedMoveDTO `json:"moves"`
}

func toPredictionDTO(position string, selfElo, oppoElo int, prediction core.Prediction) predictionDTO {
	dto := predictionDTO{
		Position: position,
		SelfElo:  selfElo,
		OppoElo:  oppoElo,
		Win:      prediction.Win,
		Draw:     prediction.Draw,
		Loss:     prediction.Loss,
		Moves:    make([]predictedMoveDTO, 0, len(prediction.Moves)),
	}
	for _, move := range prediction.Moves {
		dto.Moves = append(dto.Moves, predictedMoveDTO{
			SAN: move.SAN, UCI: move.UCI, Position: move.After, Probability: move.Probability,
		})
	}
	return dto
}

// positionParam reads the optional position parameter, the start position by
// default.
func positionParam(r *http.Request) string {
	if position := r.URL.Query().Get("position"); position != "" {
		return position
	}
	return core.StartPosition
}

func (h *handler) moves(w http.ResponseWriter, r *http.Request) {
	position := positionParam(r)
	legal, err := board.LegalMoves(position)
	if err != nil {
		badRequest(w, err.Error())
		return
	}

	body := struct {
		Position string         `json:"position"`
		Moves    []legalMoveDTO `json:"moves"`
	}{Position: position, Moves: make([]legalMoveDTO, 0, len(legal))}
	for _, move := range legal {
		body.Moves = append(body.Moves, legalMoveDTO{SAN: move.SAN, UCI: move.UCI, Position: move.After})
	}
	writeJSON(w, http.StatusOK, body)
}

func (h *handler) predictions(w http.ResponseWriter, r *http.Request) {
	if h.predictor == nil {
		writeProblem(w, http.StatusServiceUnavailable, "No prediction model",
			"the server was started without a Maia model; pass -maia with the path to a model file")
		return
	}
	selfElo, ok := eloParam(w, r, "selfElo")
	if !ok {
		return
	}
	oppoElo, ok := eloParam(w, r, "oppoElo")
	if !ok {
		return
	}

	position := positionParam(r)
	if _, err := board.Parse(position); err != nil {
		badRequest(w, err.Error())
		return
	}
	prediction, err := h.predictor.Predict(r.Context(), position, selfElo, oppoElo)
	if err != nil {
		writeProblem(w, http.StatusInternalServerError, "Prediction failure", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, toPredictionDTO(position, selfElo, oppoElo, prediction))
}

// eloParam reads a required rating. On a problem it writes the error
// response and returns false.
func eloParam(w http.ResponseWriter, r *http.Request, name string) (int, bool) {
	raw := r.URL.Query().Get(name)
	rating, err := strconv.Atoi(raw)
	if err != nil || rating < minElo || rating > maxElo {
		badRequest(w, fmt.Sprintf("%s must be a rating between %d and %d, got %q", name, minElo, maxElo, raw))
		return 0, false
	}
	return rating, true
}
