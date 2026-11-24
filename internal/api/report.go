package api

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"opposition/internal/board"
	"opposition/internal/core"
)

// What the report looks at. These are fixed for now.
const (
	reportMainLinePlies = 16
	reportMainLines     = 8
	reportWeakScore     = 0.35 // a score at or below this counts as scoring badly
	reportTrapPlies     = 24
	reportTrapMinLoss   = 100 // centipawns a reply must lose to count as going wrong
	reportTrapMinChance = 0.4
	reportTrapOwnLoss   = 50
	reportTrapReplies   = 4
	reportReplyChance   = 0.08
	fallbackElo         = 1500
)

type summaryDTO struct {
	Games int `json:"games"`
	// Wins, Draws, Losses and Score are the opponent's.
	Wins   int      `json:"wins"`
	Draws  int      `json:"draws"`
	Losses int      `json:"losses"`
	Score  *float64 `json:"score,omitempty"`
	// First and Last are the dates of their oldest and newest dated game.
	First string `json:"first,omitempty"`
	Last  string `json:"last,omitempty"`
}

type lineDTO struct {
	Line   []stepDTO `json:"line"`
	Games  int       `json:"games"`
	Wins   int       `json:"wins"`
	Draws  int       `json:"draws"`
	Losses int       `json:"losses"`
	Score  *float64  `json:"score,omitempty"`
}

type weakSpotDTO struct {
	// Line leads to Position, where you are to move; Move is what they
	// faced there and scored badly against.
	Line     []stepDTO `json:"line"`
	Position string    `json:"position"`
	Move     moveDTO   `json:"move"`
}

type replyDTO struct {
	SAN      string `json:"san"`
	UCI      string `json:"uci"`
	Position string `json:"position"`
	// Probability that a player of the opponent's rating plays this reply.
	Probability float64 `json:"probability"`
	// Loss is what the reply gives away, in centipawns.
	Loss int `json:"loss"`
	// Eval is the position after the reply, from your point of view.
	Eval evalDTO `json:"eval"`
	// Punish is the engine's answer to the reply.
	Punish string `json:"punish,omitempty"`
}

// trapDTO is a sound move of yours after which a player of the opponent's
// rating is likely to go wrong.
type trapDTO struct {
	// Line leads to Position, where you are to move.
	Line     []stepDTO `json:"line"`
	Position string    `json:"position"`
	// Games is how many of the opponent's games reached Position.
	Games int     `json:"games"`
	Move  stepDTO `json:"move"`
	// FacedGames is how often they faced Move; 0 means the engine suggested it.
	FacedGames int `json:"facedGames"`
	// OwnLoss is what Move gives up against the engine's best, in centipawns.
	OwnLoss int `json:"ownLoss"`
	// Eval is the position after Move, from your point of view.
	Eval evalDTO `json:"eval"`
	// ErrorChance is how likely a reply that loses a pawn or more is.
	ErrorChance float64 `json:"errorChance"`
	// ExpectedLoss is the probability-weighted loss in centipawns.
	ExpectedLoss float64    `json:"expectedLoss"`
	Replies      []replyDTO `json:"replies"`
}

type reportDTO struct {
	Opponent string `json:"opponent"`
	MyColor  string `json:"myColor"`
	// TheirElo and MyElo are the ratings the predictions were made for.
	TheirElo  int           `json:"theirElo"`
	MyElo     int           `json:"myElo"`
	Summary   summaryDTO    `json:"summary"`
	MainLines []lineDTO     `json:"mainLines"`
	WeakSpots []weakSpotDTO `json:"weakSpots"`
	// Mistakes is null when there is no engine, Traps when the engine or the
	// prediction model is missing.
	Mistakes *[]mistakeDTO `json:"mistakes"`
	Traps    *[]trapDTO    `json:"traps"`
}

// reportState is the report as far as it has got.
type reportState struct {
	// Status is "running", "done" or "failed".
	Status string `json:"status"`
	// Stage says what the engine is working on: "mistakes" or "traps".
	Stage  string    `json:"stage,omitempty"`
	Done   int       `json:"done"`
	Total  int       `json:"total"`
	Error  string    `json:"error,omitempty"`
	Report reportDTO `json:"report"`

	// narrative caches the write-up so asking again does not call the
	// language model again.
	narrative string
}

func toStepDTOs(line []core.Step) []stepDTO {
	steps := make([]stepDTO, 0, len(line))
	for _, step := range line {
		steps = append(steps, stepDTO{SAN: step.SAN, UCI: step.UCI, Position: step.Position})
	}
	return steps
}

func isoDate(date time.Time) string {
	if date.IsZero() {
		return ""
	}
	return date.Format("2006-01-02")
}

// scoreOf is the opponent's points per finished game, nil without one.
func scoreOf(wins, draws, losses int) *float64 {
	score, known := core.MoveStat{Wins: wins, Draws: draws, Losses: losses}.Score()
	if !known {
		return nil
	}
	return &score
}

func toSummaryDTO(summary core.Summary) summaryDTO {
	return summaryDTO{
		Games: summary.Games, Wins: summary.Wins, Draws: summary.Draws, Losses: summary.Losses,
		Score: scoreOf(summary.Wins, summary.Draws, summary.Losses),
		First: isoDate(summary.First), Last: isoDate(summary.Last),
	}
}

func toTrapDTO(trap core.Trap) trapDTO {
	dto := trapDTO{
		Line:         toStepDTOs(trap.Line),
		Position:     trap.Position,
		Games:        trap.Games,
		Move:         stepDTO{SAN: trap.Move.SAN, UCI: trap.Move.UCI, Position: trap.Move.Position},
		FacedGames:   trap.FacedGames,
		OwnLoss:      trap.OwnLoss,
		Eval:         toEvalDTO(trap.After, false), // after your move it is their turn
		ErrorChance:  trap.ErrorChance,
		ExpectedLoss: trap.ExpectedLoss,
		Replies:      make([]replyDTO, 0, len(trap.Replies)),
	}
	for _, reply := range trap.Replies {
		dto.Replies = append(dto.Replies, replyDTO{
			SAN: reply.SAN, UCI: reply.UCI, Position: reply.After,
			Probability: reply.Probability,
			Loss:        reply.Loss,
			Eval:        toEvalDTO(reply.Eval, true), // after their reply it is your turn
			Punish:      sanOf(reply.After, reply.Eval.Best),
		})
	}
	return dto
}

// rules answers the core's questions about legal moves.
type rules struct{}

func (rules) Moves(position string) ([]core.Step, error) {
	legal, err := board.LegalMoves(position)
	if err != nil {
		return nil, err
	}
	steps := make([]core.Step, 0, len(legal))
	for _, move := range legal {
		steps = append(steps, core.Step{SAN: move.SAN, UCI: move.UCI, Position: move.After})
	}
	return steps, nil
}

// reportRequest is what a report is asked for.
type reportRequest struct {
	selection
	theirElo int
	myElo    int
}

func (r reportRequest) key() string {
	return fmt.Sprintf("%s|%d|%g|%d|%d", r.opponent, r.myColor, r.halfLifeYears, r.theirElo, r.myElo)
}

// reportRequest reads the report parameters. The ratings default to the
// opponent's most recent rating in the database. On a problem it writes the
// error response and returns false.
func (h *handler) reportRequest(w http.ResponseWriter, r *http.Request) (reportRequest, bool) {
	sel, ok := h.selection(w, r)
	if !ok {
		return reportRequest{}, false
	}
	request := reportRequest{selection: sel, theirElo: h.ratingOf(sel.opponent)}
	if request.theirElo, ok = optionalElo(w, r, "theirElo", request.theirElo); !ok {
		return reportRequest{}, false
	}
	if request.myElo, ok = optionalElo(w, r, "myElo", request.theirElo); !ok {
		return reportRequest{}, false
	}
	return request, true
}

// ratingOf is the player's most recent rating in the database, or a middling
// default when the database has none.
func (h *handler) ratingOf(player string) int {
	for _, known := range core.Players(h.games, player) {
		if known.Name == player && known.Rating > 0 {
			return known.Rating
		}
	}
	return fallbackElo
}

// optionalElo reads a rating that may be left out.
func optionalElo(w http.ResponseWriter, r *http.Request, name string, fallback int) (int, bool) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return fallback, true
	}
	rating, err := strconv.Atoi(raw)
	if err != nil || rating < minElo || rating > maxElo {
		badRequest(w, fmt.Sprintf("%s must be a rating between %d and %d, got %q", name, minElo, maxElo, raw))
		return 0, false
	}
	return rating, true
}

// statistics fills in the parts of a report that need no engine.
func (h *handler) statistics(request reportRequest) reportDTO {
	theirColor := request.myColor.Opposite()
	tree := h.treeFor(request.selection)
	habits := core.ScanOptions{MinGames: defaultMinGames, MaxPlies: defaultMaxPlies}

	report := reportDTO{
		Opponent:  request.opponent,
		MyColor:   colorName(request.myColor),
		TheirElo:  request.theirElo,
		MyElo:     request.myElo,
		Summary:   toSummaryDTO(core.Summarize(h.games, request.opponent, theirColor)),
		MainLines: []lineDTO{},
		WeakSpots: []weakSpotDTO{},
	}
	lines := tree.MainLines(core.LineOptions{MinGames: defaultMinGames, MaxPlies: reportMainLinePlies, MaxLines: reportMainLines})
	for _, line := range lines {
		report.MainLines = append(report.MainLines, lineDTO{
			Line: toStepDTOs(line.Steps), Games: line.Games, Wins: line.Wins, Draws: line.Draws, Losses: line.Losses,
			Score: scoreOf(line.Wins, line.Draws, line.Losses),
		})
	}
	for _, spot := range tree.WeakSpots(request.myColor, habits, reportWeakScore) {
		report.WeakSpots = append(report.WeakSpots, weakSpotDTO{
			Line: toStepDTOs(spot.Line), Position: spot.Position, Move: toMoveDTO(spot.Move),
		})
	}
	return report
}

func (h *handler) prepReport(w http.ResponseWriter, r *http.Request) {
	request, ok := h.reportRequest(w, r)
	if !ok {
		return
	}
	start := func() reportState {
		return reportState{Status: "running", Report: h.statistics(request)}
	}
	sc := h.reports.get(request.key(), start, func(ctx context.Context, sc *scan[reportState]) {
		err := h.analyse(ctx, request, sc)
		sc.update(func(state *reportState) {
			state.Stage = ""
			if err != nil {
				state.Status, state.Error = "failed", err.Error()
				return
			}
			state.Status = "done"
		})
	})
	writeJSON(w, http.StatusOK, sc.snapshot())
}

// analyse runs the engine-backed parts of a report: recurring mistakes, then
// traps. Each needs what the server was started with; without it the section
// stays null.
func (h *handler) analyse(ctx context.Context, request reportRequest, sc *scan[reportState]) error {
	if h.engine == nil {
		return nil
	}
	theirColor := request.myColor.Opposite()
	tree := h.treeFor(request.selection)
	progress := func(done, total int) {
		sc.update(func(state *reportState) { state.Done, state.Total = done, total })
	}
	enter := func(stage string, total int) {
		sc.update(func(state *reportState) { state.Stage, state.Done, state.Total = stage, 0, total })
	}

	candidates := tree.Candidates(theirColor, core.ScanOptions{MinGames: defaultMinGames, MaxPlies: defaultMaxPlies})
	enter("mistakes", len(candidates))
	found, err := core.FindMistakes(ctx, candidates, h.engine, defaultMinLoss, progress)
	if err != nil {
		return err
	}
	mistakes := make([]mistakeDTO, 0, len(found))
	for _, mistake := range found {
		mistakes = append(mistakes, toMistakeDTO(mistake))
	}
	sc.update(func(state *reportState) { state.Report.Mistakes = &mistakes })

	if h.predictor == nil {
		return nil
	}
	turns := tree.MyTurns(request.myColor, core.ScanOptions{MinGames: defaultMinGames, MaxPlies: reportTrapPlies})
	enter("traps", len(turns))
	sprung, err := core.FindTraps(ctx, turns, rules{}, h.engine, h.predictor, core.TrapOptions{
		TheirElo:       request.theirElo,
		MyElo:          request.myElo,
		MinLoss:        reportTrapMinLoss,
		MinChance:      reportTrapMinChance,
		MaxOwnLoss:     reportTrapOwnLoss,
		MaxReplies:     reportTrapReplies,
		MinReplyChance: reportReplyChance,
	}, progress)
	if err != nil {
		return err
	}
	traps := make([]trapDTO, 0, len(sprung))
	for _, trap := range sprung {
		traps = append(traps, toTrapDTO(trap))
	}
	sc.update(func(state *reportState) { state.Report.Traps = &traps })
	return nil
}

// narrative has the finished report written up in prose.
func (h *handler) narrative(w http.ResponseWriter, r *http.Request) {
	if h.narrator == nil {
		writeProblem(w, http.StatusServiceUnavailable, "No language model",
			"the server was started without a language model; pass -llm-url and -llm-model")
		return
	}
	request, ok := h.reportRequest(w, r)
	if !ok {
		return
	}
	sc, exists := h.reports.lookup(request.key())
	if !exists || sc.snapshot().Status != "done" {
		writeProblem(w, http.StatusConflict, "Report not ready",
			"the report for these settings has not finished; request it first and wait until its status is done")
		return
	}

	state := sc.snapshot()
	if state.narrative == "" {
		text, err := h.narrator.Narrate(r.Context(), renderFacts(state.Report))
		if err != nil {
			writeProblem(w, http.StatusBadGateway, "Language model failure", err.Error())
			return
		}
		sc.update(func(state *reportState) { state.narrative = text })
		state.narrative = text
	}
	writeJSON(w, http.StatusOK, struct {
		Text string `json:"text"`
	}{state.narrative})
}
