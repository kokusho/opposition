package api_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"opposition/internal/api"
	"opposition/internal/core"
)

type lineJSON []struct {
	SAN string `json:"san"`
}

func (l lineJSON) String() string {
	var sans []string
	for _, step := range l {
		sans = append(sans, step.SAN)
	}
	return strings.Join(sans, " ")
}

type reportJSON struct {
	Status string `json:"status"`
	Stage  string `json:"stage"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	Error  string `json:"error"`
	Report struct {
		Opponent string `json:"opponent"`
		MyColor  string `json:"myColor"`
		TheirElo int    `json:"theirElo"`
		MyElo    int    `json:"myElo"`
		Summary  struct {
			Games  int      `json:"games"`
			Wins   int      `json:"wins"`
			Draws  int      `json:"draws"`
			Losses int      `json:"losses"`
			Score  *float64 `json:"score"`
			First  string   `json:"first"`
			Last   string   `json:"last"`
		} `json:"summary"`
		MainLines []struct {
			Line  lineJSON `json:"line"`
			Games int      `json:"games"`
		} `json:"mainLines"`
		WeakSpots []struct {
			Line lineJSON `json:"line"`
			Move struct {
				SAN   string   `json:"san"`
				Games int      `json:"games"`
				Score *float64 `json:"score"`
			} `json:"move"`
		} `json:"weakSpots"`
		Mistakes *[]struct {
			Move struct {
				SAN string `json:"san"`
			} `json:"move"`
			Loss int `json:"loss"`
		} `json:"mistakes"`
		Traps *[]struct {
			Line lineJSON `json:"line"`
			Move struct {
				SAN string `json:"san"`
			} `json:"move"`
			FacedGames   int      `json:"facedGames"`
			Eval         evalJSON `json:"eval"`
			ErrorChance  float64  `json:"errorChance"`
			ExpectedLoss float64  `json:"expectedLoss"`
			Replies      []struct {
				SAN         string   `json:"san"`
				Probability float64  `json:"probability"`
				Loss        int      `json:"loss"`
				Eval        evalJSON `json:"eval"`
				Punish      string   `json:"punish"`
			} `json:"replies"`
		} `json:"traps"`
	} `json:"report"`
}

// awaitReport polls the report until it is no longer running.
func awaitReport(t *testing.T, handler http.Handler, query url.Values) reportJSON {
	t.Helper()
	for attempt := 0; attempt < 300; attempt++ {
		body := decode[reportJSON](t, get(t, handler, "/api/v1/report", query), http.StatusOK)
		if body.Status != "running" {
			return body
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("report did not finish")
	return reportJSON{}
}

func TestReportWithoutAnEngineHasTheStatisticsAndNoScans(t *testing.T) {
	body := awaitReport(t, sampleAPI(t), petrovAs("white"))

	if body.Status != "done" || body.Report.Opponent != "Petrov, Ivan" || body.Report.MyColor != "white" {
		t.Fatalf("status %q for %q as %q", body.Status, body.Report.Opponent, body.Report.MyColor)
	}
	summary := body.Report.Summary
	if summary.Games != 8 || summary.Wins != 2 || summary.Draws != 3 || summary.Losses != 3 ||
		summary.First != "2019-01-01" || summary.Last != "2026-03-16" {
		t.Errorf("summary = %+v, want 8 games +2 =3 -3 from 2019-01-01 to 2026-03-16", summary)
	}
	if len(body.Report.MainLines) == 0 || body.Report.MainLines[0].Games != 3 ||
		body.Report.MainLines[0].Line.String() != "e4 c5 Nf3 d6 d4 cxd4 Nxd4 Nf6 Nc3 a6" {
		t.Errorf("main lines = %+v, want the Najdorf (3 games) first", body.Report.MainLines)
	}
	// Petrov scored 2/6 against 1.e4 (better against the other first moves)
	// and 0/2 against 2.c3 (better against 2.Nf3).
	spots := body.Report.WeakSpots
	if len(spots) != 2 || spots[0].Move.SAN != "e4" || spots[0].Move.Games != 6 ||
		spots[1].Move.SAN != "c3" || spots[1].Line.String() != "e4 c5" || spots[1].Move.Games != 2 {
		t.Errorf("weak spots = %+v, want 1.e4 (6 games) and 2.c3 after 1.e4 c5 (2 games)", spots)
	}
	if body.Report.Mistakes != nil || body.Report.Traps != nil {
		t.Error("mistakes and traps should be null without an engine")
	}
}

func TestReportWithAnEngineListsRecurringMistakes(t *testing.T) {
	plain := sampleAPI(t)
	c3Line := []string{"e4", "c5", "c3", "d5", "exd5", "Qxd5", "d4", "Nf6", "Nf3"}
	engine := scriptedEngine{
		positionAfter(t, plain, "white", append(c3Line, "Bg4")...): {Centipawns: 140},
	}

	body := awaitReport(t, sampleAPIWith(t, engine), petrovAs("white"))

	if body.Status != "done" || body.Report.Mistakes == nil {
		t.Fatalf("status %q, error %q, mistakes %v", body.Status, body.Error, body.Report.Mistakes)
	}
	if mistakes := *body.Report.Mistakes; len(mistakes) != 1 || mistakes[0].Move.SAN != "Bg4" || mistakes[0].Loss != 140 {
		t.Errorf("mistakes = %+v, want Bg4 losing 140", mistakes)
	}
	if body.Report.Traps != nil {
		t.Error("traps should be null without a prediction model")
	}
}

// scriptedPredictor answers from a table of position -> predicted replies.
type scriptedPredictor struct {
	replies          map[string][]core.PredictedMove
	selfElo, oppoElo int
}

func (s *scriptedPredictor) Predict(_ context.Context, position string, selfElo, oppoElo int) (core.Prediction, error) {
	s.selfElo, s.oppoElo = selfElo, oppoElo
	return core.Prediction{Moves: s.replies[position]}, nil
}

func TestReportWithEngineAndModelListsTraps(t *testing.T) {
	plain := sampleAPI(t)
	afterC3 := positionAfter(t, plain, "white", "e4", "c5", "c3")
	afterD5 := positionAfter(t, plain, "white", "e4", "c5", "c3", "d5")
	engine := scriptedEngine{
		afterC3: {Centipawns: -20},               // Black to move is slightly worse
		afterD5: {Centipawns: 200, Best: "e4d5"}, // after the reply White is two pawns up
	}
	predictor := &scriptedPredictor{replies: map[string][]core.PredictedMove{
		afterC3: {{SAN: "d5", UCI: "d7d5", After: afterD5, Probability: 0.75}},
	}}
	query := petrovAs("white")
	query.Set("theirElo", "2100")
	query.Set("myElo", "1900")

	body := awaitReport(t, sampleAPIFull(t, engine, predictor), query)

	if body.Status != "done" || body.Report.Traps == nil {
		t.Fatalf("status %q, error %q, traps %v", body.Status, body.Error, body.Report.Traps)
	}
	traps := *body.Report.Traps
	if len(traps) != 1 {
		t.Fatalf("got %d traps, want 1: %+v", len(traps), traps)
	}
	trap := traps[0]
	if trap.Move.SAN != "c3" || trap.Line.String() != "e4 c5" || trap.FacedGames != 2 || trap.Eval.Text != "+0.20" {
		t.Errorf("trap = %+v, want 2.c3 after 1.e4 c5, faced twice, +0.20 for me", trap)
	}
	if trap.ErrorChance != 0.75 || trap.ExpectedLoss != 0.75*180 {
		t.Errorf("error chance %v, expected loss %v; want 0.75 and 135", trap.ErrorChance, trap.ExpectedLoss)
	}
	if len(trap.Replies) != 1 || trap.Replies[0].SAN != "d5" || trap.Replies[0].Loss != 180 ||
		trap.Replies[0].Eval.Text != "+2.00" || trap.Replies[0].Punish != "exd5" {
		t.Errorf("replies = %+v, want d5 losing 180, +2.00 for me, punished by exd5", trap.Replies)
	}
	if body.Report.TheirElo != 2100 || body.Report.MyElo != 1900 || predictor.selfElo != 2100 || predictor.oppoElo != 1900 {
		t.Errorf("ratings: report %d/%d, model asked with %d/%d; want 2100/1900 both",
			body.Report.TheirElo, body.Report.MyElo, predictor.selfElo, predictor.oppoElo)
	}
}

func TestReportUsesTheDatabaseRatingWhenNoneIsGiven(t *testing.T) {
	body := awaitReport(t, sampleAPI(t), petrovAs("white"))

	if body.Report.TheirElo != 2105 || body.Report.MyElo != 2105 {
		t.Errorf("ratings = %d/%d, want 2105/2105 from Petrov's latest rated game", body.Report.TheirElo, body.Report.MyElo)
	}
}

func TestReportRejectsBadParameters(t *testing.T) {
	handler := sampleAPI(t)
	decode[problem](t, get(t, handler, "/api/v1/report", url.Values{"myColor": {"white"}}), http.StatusBadRequest)

	bad := petrovAs("white")
	bad.Set("theirElo", "enormous")
	decode[problem](t, get(t, handler, "/api/v1/report", bad), http.StatusBadRequest)

	unknown := url.Values{"opponent": {"Nobody, Nemo"}, "myColor": {"white"}}
	decode[problem](t, get(t, handler, "/api/v1/report", unknown), http.StatusNotFound)
}

// recordingNarrator returns a fixed text and remembers the facts it was given.
type recordingNarrator struct{ facts string }

func (r *recordingNarrator) Narrate(_ context.Context, facts string) (string, error) {
	r.facts = facts
	return "Play 2.c3.", nil
}

func post(t *testing.T, handler http.Handler, path string, query url.Values) *http.Response {
	t.Helper()
	server := newServer(t, handler)
	response, err := http.Post(server+path+"?"+query.Encode(), "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { response.Body.Close() })
	return response
}

func TestNarrativeHandsTheFinishedReportToTheWriter(t *testing.T) {
	narrator := &recordingNarrator{}
	handler := sampleAPIConfigured(t, func(config *api.Config) { config.Narrator = narrator })
	awaitReport(t, handler, petrovAs("white"))

	response := post(t, handler, "/api/v1/report/narrative", petrovAs("white"))

	body := decodeBody[struct {
		Text string `json:"text"`
	}](t, response, http.StatusOK)
	if body.Text != "Play 2.c3." {
		t.Errorf("text = %q", body.Text)
	}
	for _, want := range []string{"Petrov, Ivan", "8 games", "1. e4 c5 2. Nf3 d6", "2. c3"} {
		if !strings.Contains(narrator.facts, want) {
			t.Errorf("facts given to the writer lack %q:\n%s", want, narrator.facts)
		}
	}
}

func TestNarrativeNeedsAWriterAndAFinishedReport(t *testing.T) {
	withoutWriter := sampleAPI(t)
	awaitReport(t, withoutWriter, petrovAs("white"))
	if response := post(t, withoutWriter, "/api/v1/report/narrative", petrovAs("white")); response.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("without a writer: status %d, want 503", response.StatusCode)
	}

	withWriter := sampleAPIConfigured(t, func(config *api.Config) { config.Narrator = &recordingNarrator{} })
	if response := post(t, withWriter, "/api/v1/report/narrative", petrovAs("black")); response.StatusCode != http.StatusConflict {
		t.Errorf("before the report was built: status %d, want 409", response.StatusCode)
	}
}

func TestDatabaseSaysWhetherAWriteUpIsAvailable(t *testing.T) {
	type body struct {
		Narrative bool `json:"narrative"`
	}
	with := sampleAPIConfigured(t, func(config *api.Config) { config.Narrator = &recordingNarrator{} })

	if decode[body](t, get(t, sampleAPI(t), "/api/v1/database", nil), http.StatusOK).Narrative {
		t.Error("narrative flag set without a writer")
	}
	if !decode[body](t, get(t, with, "/api/v1/database", nil), http.StatusOK).Narrative {
		t.Error("narrative flag not set with a writer")
	}
}
