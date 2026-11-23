package api_test

import (
	"context"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"opposition/internal/core"
)

// scriptedEngine answers from a table and calls every other position level.
type scriptedEngine map[string]core.Eval

func (s scriptedEngine) Evaluate(_ context.Context, position string) (core.Eval, error) {
	return s[position], nil
}

// positionAfter walks the tree endpoint along the given moves and returns the
// key of the position reached, so tests need no hand-written FENs.
func positionAfter(t *testing.T, handler http.Handler, myColor string, sans ...string) string {
	t.Helper()
	position := ""
	for _, san := range sans {
		query := petrovAs(myColor)
		query.Set("halfLifeYears", "0")
		if position != "" {
			query.Set("position", position)
		}
		body := decode[struct {
			Moves []struct {
				SAN      string `json:"san"`
				Position string `json:"position"`
			} `json:"moves"`
		}](t, get(t, handler, "/api/v1/tree", query), http.StatusOK)
		next := ""
		for _, move := range body.Moves {
			if move.SAN == san {
				next = move.Position
			}
		}
		if next == "" {
			t.Fatalf("move %s not found after %v", san, sans)
		}
		position = next
	}
	return position
}

type evalJSON struct {
	Text       string `json:"text"`
	Centipawns int    `json:"centipawns"`
}

func TestDatabaseSaysWhetherAnEngineIsAvailable(t *testing.T) {
	type body struct {
		Engine bool `json:"engine"`
	}
	without := decode[body](t, get(t, sampleAPI(t), "/api/v1/database", nil), http.StatusOK)
	with := decode[body](t, get(t, sampleAPIWith(t, scriptedEngine{}), "/api/v1/database", nil), http.StatusOK)

	if without.Engine || !with.Engine {
		t.Errorf("engine flag = %v without and %v with an engine; want false and true", without.Engine, with.Engine)
	}
}

func TestAnalysisEvaluatesThePositionAndEachMoveFromYourSide(t *testing.T) {
	afterC5 := positionAfter(t, sampleAPI(t), "white", "e4", "c5")
	engine := scriptedEngine{
		afterE4: {Centipawns: -30, Best: "e7e5", Depth: 18}, // Black to move is 0.30 down
		afterC5: {Centipawns: 80, Best: "g1f3", Depth: 18},  // White to move is 0.80 up
	}
	query := petrovAs("white")
	query.Set("position", afterE4)

	body := decode[struct {
		Position string   `json:"position"`
		Eval     evalJSON `json:"eval"`
		Best     string   `json:"best"`
		Moves    []struct {
			UCI  string   `json:"uci"`
			Eval evalJSON `json:"eval"`
			Loss int      `json:"loss"`
		} `json:"moves"`
	}](t, get(t, sampleAPIWith(t, engine), "/api/v1/analysis", query), http.StatusOK)

	if body.Position != afterE4 || body.Eval.Text != "+0.30" || body.Best != "e5" {
		t.Errorf("got position %q, eval %q, best %q; want afterE4, +0.30, e5", body.Position, body.Eval.Text, body.Best)
	}
	if len(body.Moves) != 1 {
		t.Fatalf("got %d moves, want 1 (c5)", len(body.Moves))
	}
	if c5 := body.Moves[0]; c5.UCI != "c7c5" || c5.Eval.Text != "+0.80" || c5.Loss != 50 {
		t.Errorf("c5 = %+v, want eval +0.80 and a loss of 50", c5)
	}
}

func TestAnalysisNeedsAnEngine(t *testing.T) {
	recorder := get(t, sampleAPI(t), "/api/v1/analysis", petrovAs("white"))

	body := decode[problem](t, recorder, http.StatusServiceUnavailable)
	if body.Detail == "" {
		t.Error("want a detail explaining that no engine is configured")
	}
}

func TestAnalysisRejectsBadParameters(t *testing.T) {
	recorder := get(t, sampleAPIWith(t, scriptedEngine{}), "/api/v1/analysis", url.Values{"myColor": {"white"}})

	decode[problem](t, recorder, http.StatusBadRequest)
}

type mistakesJSON struct {
	Status string `json:"status"`
	Done   int    `json:"done"`
	Total  int    `json:"total"`
	Error  string `json:"error"`
	Items  []struct {
		Line []struct {
			SAN      string `json:"san"`
			Position string `json:"position"`
		} `json:"line"`
		Position string `json:"position"`
		Move     struct {
			SAN   string `json:"san"`
			Games int    `json:"games"`
		} `json:"move"`
		Loss       int      `json:"loss"`
		Best       string   `json:"best"`
		Eval       evalJSON `json:"eval"`
		Refutation []string `json:"refutation"`
	} `json:"items"`
}

// awaitMistakes polls the scan until it is no longer running.
func awaitMistakes(t *testing.T, handler http.Handler, query url.Values) mistakesJSON {
	t.Helper()
	for attempt := 0; attempt < 200; attempt++ {
		body := decode[mistakesJSON](t, get(t, handler, "/api/v1/mistakes", query), http.StatusOK)
		if body.Status != "running" {
			return body
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("scan did not finish")
	return mistakesJSON{}
}

func TestMistakesScanReportsTheHabitualMoveTheEngineDislikes(t *testing.T) {
	plain := sampleAPI(t)
	c3Line := []string{"e4", "c5", "c3", "d5", "exd5", "Qxd5", "d4", "Nf6", "Nf3"}
	beforeBg4 := positionAfter(t, plain, "white", c3Line...)
	afterBg4 := positionAfter(t, plain, "white", append(c3Line, "Bg4")...)
	engine := scriptedEngine{
		beforeBg4: {Centipawns: -10, Best: "e7e6"},
		afterBg4:  {Centipawns: 140, Best: "f1e2", Line: []string{"f1e2", "e7e6"}},
	}

	body := awaitMistakes(t, sampleAPIWith(t, engine), petrovAs("white"))

	if body.Status != "done" || body.Total == 0 || body.Done != body.Total {
		t.Fatalf("status %q, %d of %d, error %q; want done with everything analysed", body.Status, body.Done, body.Total, body.Error)
	}
	if len(body.Items) != 1 {
		t.Fatalf("got %d mistakes, want 1", len(body.Items))
	}
	item := body.Items[0]
	var line []string
	for _, step := range item.Line {
		line = append(line, step.SAN)
	}
	if strings.Join(line, " ") != strings.Join(c3Line, " ") || item.Position != beforeBg4 {
		t.Errorf("line = %v at %q, want the c3 Sicilian up to 5.Nf3", line, item.Position)
	}
	if item.Move.SAN != "Bg4" || item.Move.Games != 2 || item.Loss != 130 || item.Best != "e6" {
		t.Errorf("got %s (%d games) losing %d with %q preferred; want Bg4, 2 games, 130, e6",
			item.Move.SAN, item.Move.Games, item.Loss, item.Best)
	}
	if item.Eval.Text != "+1.40" || strings.Join(item.Refutation, " ") != "Be2 e6" {
		t.Errorf("eval %q, refutation %v; want +1.40 and Be2 e6", item.Eval.Text, item.Refutation)
	}
}

func TestMistakesScanHonoursTheThresholds(t *testing.T) {
	plain := sampleAPI(t)
	c3Line := []string{"e4", "c5", "c3", "d5", "exd5", "Qxd5", "d4", "Nf6", "Nf3"}
	engine := scriptedEngine{
		positionAfter(t, plain, "white", append(c3Line, "Bg4")...): {Centipawns: 140},
	}
	strict := petrovAs("white")
	strict.Set("minLoss", "200")
	rare := petrovAs("white")
	rare.Set("minGames", "3")

	handler := sampleAPIWith(t, engine)

	if body := awaitMistakes(t, handler, strict); len(body.Items) != 0 {
		t.Errorf("minLoss 200: got %d mistakes, want none for a 140 loss", len(body.Items))
	}
	if body := awaitMistakes(t, handler, rare); len(body.Items) != 0 {
		t.Errorf("minGames 3: got %d mistakes, want none for a move played twice", len(body.Items))
	}
}

func TestMistakesNeedsAnEngine(t *testing.T) {
	decode[problem](t, get(t, sampleAPI(t), "/api/v1/mistakes", petrovAs("white")), http.StatusServiceUnavailable)
}

func TestMistakesRejectsBadParameters(t *testing.T) {
	handler := sampleAPIWith(t, scriptedEngine{})
	for _, name := range []string{"minGames", "minLoss", "maxPlies"} {
		query := petrovAs("white")
		query.Set(name, "lots")

		decode[problem](t, get(t, handler, "/api/v1/mistakes", query), http.StatusBadRequest)
	}
}
