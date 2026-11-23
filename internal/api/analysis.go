package api

import (
	"fmt"
	"net/http"
	"strconv"

	"opposition/internal/core"
	"opposition/internal/pgn"
)

// maxAnalysedMoves caps how many moves of one position are sent to the engine.
const maxAnalysedMoves = 12

// evalDTO is an engine evaluation from your point of view.
type evalDTO struct {
	// Text is the usual notation: "+0.31" in pawns, "#3" when you mate in
	// three, "-#3" when you get mated.
	Text string `json:"text"`
	// Centipawns is your advantage, capped at 1000 either way, for comparing.
	Centipawns int `json:"centipawns"`
	Depth      int `json:"depth"`
}

type analysisDTO struct {
	Position string  `json:"position"`
	Eval     evalDTO `json:"eval"`
	// Best is the engine's preferred move in this position.
	Best  string            `json:"best,omitempty"`
	Moves []moveAnalysisDTO `json:"moves"`
}

type moveAnalysisDTO struct {
	UCI  string  `json:"uci"`
	Eval evalDTO `json:"eval"`
	// Loss is how many centipawns the move gives away for whoever plays it.
	Loss int `json:"loss"`
}

// toEvalDTO turns an evaluation, which engines give for the side to move,
// into one for you. yourMove says whether you are the side to move.
func toEvalDTO(eval core.Eval, yourMove bool) evalDTO {
	sign := 1
	if !yourMove {
		sign = -1
	}
	dto := evalDTO{Centipawns: sign * eval.Value(), Depth: eval.Depth}

	switch {
	case eval.IsMate:
		moves := eval.MateIn
		if moves < 0 {
			moves = -moves
		}
		dto.Text = "#"
		if moves > 0 {
			dto.Text += strconv.Itoa(moves)
		}
		// A positive MateIn means the side to move delivers the mate.
		if youMate := (eval.MateIn > 0) == yourMove; !youMate {
			dto.Text = "-" + dto.Text
		}
	case eval.Centipawns == 0:
		dto.Text = "0.00"
	default:
		dto.Text = fmt.Sprintf("%+.2f", float64(sign*eval.Centipawns)/100)
	}
	return dto
}

// sanOf converts one engine move to standard notation, falling back to the
// engine's own notation if the move cannot be played in the position.
func sanOf(position, uci string) string {
	if uci == "" {
		return ""
	}
	sans, err := pgn.SANLine(position, []string{uci})
	if err != nil {
		return uci
	}
	return sans[0]
}

func (h *handler) requireEngine(w http.ResponseWriter) bool {
	if h.engine == nil {
		writeProblem(w, http.StatusServiceUnavailable, "No chess engine",
			"the server was started without a chess engine; install Stockfish or pass -engine")
		return false
	}
	return true
}

func (h *handler) analysis(w http.ResponseWriter, r *http.Request) {
	if !h.requireEngine(w) {
		return
	}
	sel, ok := h.selection(w, r)
	if !ok {
		return
	}

	yourMove := core.SideToMove(sel.position) == sel.myColor
	before, err := h.engine.Evaluate(r.Context(), sel.position)
	if err != nil {
		engineFailed(w, err)
		return
	}

	moves := h.treeFor(sel).Node(sel.position).Moves
	moves = moves[:min(len(moves), maxAnalysedMoves)]
	body := analysisDTO{
		Position: sel.position,
		Eval:     toEvalDTO(before, yourMove),
		Best:     sanOf(sel.position, before.Best),
		Moves:    make([]moveAnalysisDTO, 0, len(moves)),
	}
	for _, move := range moves {
		after, err := h.engine.Evaluate(r.Context(), move.After)
		if err != nil {
			engineFailed(w, err)
			return
		}
		body.Moves = append(body.Moves, moveAnalysisDTO{
			UCI:  move.UCI,
			Eval: toEvalDTO(after, !yourMove),
			Loss: core.Loss(before, after),
		})
	}
	writeJSON(w, http.StatusOK, body)
}

func engineFailed(w http.ResponseWriter, err error) {
	writeProblem(w, http.StatusInternalServerError, "Engine failure", err.Error())
}
