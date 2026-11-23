package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"opposition/internal/api"
	"opposition/internal/core"
	"opposition/internal/pgn"
)

const afterE4 = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -"

type node struct {
	Position   string `json:"position"`
	SideToMove string `json:"sideToMove"`
	Mover      string `json:"mover"`
	Games      int    `json:"games"`
	Moves      []struct {
		SAN    string  `json:"san"`
		Games  int     `json:"games"`
		Weight float64 `json:"weight"`
	} `json:"moves"`
}

type problem struct {
	Title  string `json:"title"`
	Detail string `json:"detail"`
	Status int    `json:"status"`
}

// sampleAPIWith serves testdata/sample.pgn with the clock fixed to 2026-11-23
// and the given engine (nil for none).
func sampleAPIWith(t *testing.T, engine core.Evaluator) http.Handler {
	t.Helper()
	file, err := os.Open("../../testdata/sample.pgn")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	games, report, err := pgn.Load(file)
	if err != nil {
		t.Fatal(err)
	}
	now := func() time.Time { return time.Date(2026, time.November, 23, 0, 0, 0, 0, time.UTC) }
	return api.NewHandler(games, report, now, engine)
}

// sampleAPI serves the sample database without a chess engine.
func sampleAPI(t *testing.T) http.Handler {
	t.Helper()
	return sampleAPIWith(t, nil)
}

func get(t *testing.T, handler http.Handler, path string, query url.Values) *httptest.ResponseRecorder {
	t.Helper()
	target := path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, target, nil))
	return recorder
}

func decode[T any](t *testing.T, recorder *httptest.ResponseRecorder, wantStatus int) T {
	t.Helper()
	if recorder.Code != wantStatus {
		t.Fatalf("status = %d, want %d; body: %s", recorder.Code, wantStatus, recorder.Body)
	}
	var value T
	if err := json.Unmarshal(recorder.Body.Bytes(), &value); err != nil {
		t.Fatalf("decoding %s: %v", recorder.Body, err)
	}
	return value
}

func petrovAs(myColor string) url.Values {
	return url.Values{"opponent": {"Petrov, Ivan"}, "myColor": {myColor}}
}

func TestDatabaseReportsWhatWasLoaded(t *testing.T) {
	recorder := get(t, sampleAPI(t), "/api/v1/database", nil)

	if got := recorder.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", got)
	}
	body := decode[struct {
		Games   int   `json:"games"`
		Skipped []any `json:"skipped"`
	}](t, recorder, http.StatusOK)
	if body.Games != 12 || body.Skipped == nil || len(body.Skipped) != 0 {
		t.Errorf("got %+v, want 12 games and an empty skipped list", body)
	}
}

func TestPlayersSearchReturnsMatchingNamesWithGameCounts(t *testing.T) {
	recorder := get(t, sampleAPI(t), "/api/v1/players", url.Values{"query": {"PETRO"}})

	want := `{"total":1,"items":[{"name":"Petrov, Ivan","games":12}]}`
	if got := recorder.Body.String(); recorder.Code != http.StatusOK || got != want+"\n" {
		t.Errorf("status %d, body %s; want 200 and %s", recorder.Code, got, want)
	}
}

func TestPlayersListIsCappedByLimitButReportsTheTotal(t *testing.T) {
	recorder := get(t, sampleAPI(t), "/api/v1/players", url.Values{"limit": {"2"}})

	body := decode[struct {
		Total int `json:"total"`
		Items []struct {
			Name string `json:"name"`
		} `json:"items"`
	}](t, recorder, http.StatusOK)
	if body.Total != 11 || len(body.Items) != 2 || body.Items[0].Name != "Petrov, Ivan" {
		t.Errorf("got total %d and items %+v; want total 11, 2 items, Petrov first", body.Total, body.Items)
	}
}

func TestTreeStartsAtTheInitialPositionWithTheOpponentOnTheOtherSide(t *testing.T) {
	recorder := get(t, sampleAPI(t), "/api/v1/tree", petrovAs("white"))

	body := decode[node](t, recorder, http.StatusOK)
	if body.Mover != "you" || body.SideToMove != "white" || body.Games != 8 {
		t.Errorf("got mover %q, side %q, %d games; want you, white, 8 (Petrov's games as Black)",
			body.Mover, body.SideToMove, body.Games)
	}
	if len(body.Moves) != 3 || body.Moves[0].SAN != "e4" || body.Moves[0].Games != 6 {
		t.Errorf("moves = %+v, want e4 (6 games) first of 3", body.Moves)
	}
}

func TestTreeFollowsThePositionParameter(t *testing.T) {
	query := petrovAs("white")
	query.Set("position", afterE4)

	body := decode[node](t, get(t, sampleAPI(t), "/api/v1/tree", query), http.StatusOK)

	if body.Position != afterE4 || body.Mover != "opponent" {
		t.Errorf("got position %q, mover %q", body.Position, body.Mover)
	}
	if len(body.Moves) != 1 || body.Moves[0].SAN != "c5" {
		t.Errorf("moves = %+v, want only c5", body.Moves)
	}
}

func TestTreeDiscountsOldGamesWithATwoYearHalfLifeByDefault(t *testing.T) {
	body := decode[node](t, get(t, sampleAPI(t), "/api/v1/tree", petrovAs("white")), http.StatusOK)

	e4 := body.Moves[0]
	if e4.Weight >= float64(e4.Games) || e4.Weight <= 0 {
		t.Errorf("e4 weight = %v for %d games, want a discounted positive weight", e4.Weight, e4.Games)
	}
}

func TestTreeCountsEveryGameFullyWithHalfLifeZero(t *testing.T) {
	query := petrovAs("white")
	query.Set("halfLifeYears", "0")

	body := decode[node](t, get(t, sampleAPI(t), "/api/v1/tree", query), http.StatusOK)

	if e4 := body.Moves[0]; e4.Weight != float64(e4.Games) {
		t.Errorf("e4 weight = %v, want %d", e4.Weight, e4.Games)
	}
}

func TestTreeRejectsBadParameters(t *testing.T) {
	handler := sampleAPI(t)
	cases := map[string]url.Values{
		"missing opponent":       {"myColor": {"white"}},
		"missing colour":         {"opponent": {"Petrov, Ivan"}},
		"unknown colour":         {"opponent": {"Petrov, Ivan"}, "myColor": {"green"}},
		"unreadable half-life":   {"opponent": {"Petrov, Ivan"}, "myColor": {"white"}, "halfLifeYears": {"soon"}},
		"negative half-life":     {"opponent": {"Petrov, Ivan"}, "myColor": {"white"}, "halfLifeYears": {"-1"}},
		"not a number half-life": {"opponent": {"Petrov, Ivan"}, "myColor": {"white"}, "halfLifeYears": {"NaN"}},
	}
	for name, query := range cases {
		recorder := get(t, handler, "/api/v1/tree", query)

		if got := recorder.Header().Get("Content-Type"); got != "application/problem+json" {
			t.Errorf("%s: Content-Type = %q, want application/problem+json", name, got)
		}
		body := decode[problem](t, recorder, http.StatusBadRequest)
		if body.Status != http.StatusBadRequest || body.Title == "" || body.Detail == "" {
			t.Errorf("%s: problem = %+v, want status 400 with title and detail", name, body)
		}
	}
}

func TestTreeForAPlayerNotInTheDatabaseIsNotFound(t *testing.T) {
	query := url.Values{"opponent": {"Nobody, Nemo"}, "myColor": {"white"}}

	body := decode[problem](t, get(t, sampleAPI(t), "/api/v1/tree", query), http.StatusNotFound)

	if body.Status != http.StatusNotFound || body.Detail == "" {
		t.Errorf("problem = %+v", body)
	}
}

func TestPlayersRejectsAnUnreadableLimit(t *testing.T) {
	recorder := get(t, sampleAPI(t), "/api/v1/players", url.Values{"limit": {"many"}})

	decode[problem](t, recorder, http.StatusBadRequest)
}
