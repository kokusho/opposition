package api_test

import (
	"context"
	"net/http"
	"net/url"
	"testing"

	"opposition/internal/core"
)

// fakePredictor returns a fixed prediction and remembers what it was asked.
type fakePredictor struct {
	position         string
	selfElo, oppoElo int
}

func (f *fakePredictor) Predict(_ context.Context, position string, selfElo, oppoElo int) (core.Prediction, error) {
	f.position, f.selfElo, f.oppoElo = position, selfElo, oppoElo
	return core.Prediction{
		Moves: []core.PredictedMove{
			{SAN: "c5", UCI: "c7c5", After: "after c5", Probability: 0.7},
			{SAN: "e5", UCI: "e7e5", After: "after e5", Probability: 0.3},
		},
		Win: 0.4, Draw: 0.1, Loss: 0.5,
	}, nil
}

type movesJSON struct {
	Position string `json:"position"`
	Moves    []struct {
		SAN      string `json:"san"`
		UCI      string `json:"uci"`
		Position string `json:"position"`
	} `json:"moves"`
}

func TestMovesListsTheLegalMovesOfAPosition(t *testing.T) {
	body := decode[movesJSON](t, get(t, sampleAPI(t), "/api/v1/moves", url.Values{"position": {afterE4}}), http.StatusOK)

	if body.Position != afterE4 || len(body.Moves) != 20 {
		t.Fatalf("got %d moves for %q, want 20 for the position after 1.e4", len(body.Moves), body.Position)
	}
	found := false
	for _, move := range body.Moves {
		if move.UCI == "c7c5" {
			found = move.SAN == "c5" && move.Position == "rnbqkbnr/pp1ppppp/8/2p5/4P3/8/PPPP1PPP/RNBQKBNR w KQkq -"
		}
	}
	if !found {
		t.Errorf("c7c5 missing or wrong in %+v", body.Moves)
	}
}

func TestMovesStartsFromTheInitialPositionByDefault(t *testing.T) {
	body := decode[movesJSON](t, get(t, sampleAPI(t), "/api/v1/moves", nil), http.StatusOK)

	if body.Position != core.StartPosition || len(body.Moves) != 20 {
		t.Errorf("got %d moves for %q", len(body.Moves), body.Position)
	}
}

func TestMovesRejectsAnUnreadablePosition(t *testing.T) {
	decode[problem](t, get(t, sampleAPI(t), "/api/v1/moves", url.Values{"position": {"nonsense"}}), http.StatusBadRequest)
}

func TestDatabaseSaysWhetherMovePredictionIsAvailable(t *testing.T) {
	type body struct {
		Predictions bool `json:"predictions"`
	}
	without := decode[body](t, get(t, sampleAPI(t), "/api/v1/database", nil), http.StatusOK)
	with := decode[body](t, get(t, sampleAPIFull(t, nil, &fakePredictor{}), "/api/v1/database", nil), http.StatusOK)

	if without.Predictions || !with.Predictions {
		t.Errorf("predictions flag = %v without and %v with a model; want false and true", without.Predictions, with.Predictions)
	}
}

func TestPredictionsPassTheRatingsOnAndReturnProbabilities(t *testing.T) {
	predictor := &fakePredictor{}
	query := url.Values{"position": {afterE4}, "selfElo": {"1900"}, "oppoElo": {"2100"}}

	recorder := get(t, sampleAPIFull(t, nil, predictor), "/api/v1/predictions", query)

	want := `{"position":"` + afterE4 + `","selfElo":1900,"oppoElo":2100,"win":0.4,"draw":0.1,"loss":0.5,` +
		`"moves":[{"san":"c5","uci":"c7c5","position":"after c5","probability":0.7},` +
		`{"san":"e5","uci":"e7e5","position":"after e5","probability":0.3}]}`
	if got := recorder.Body.String(); recorder.Code != http.StatusOK || got != want+"\n" {
		t.Errorf("status %d, body %s\nwant %s", recorder.Code, got, want)
	}
	if predictor.position != afterE4 || predictor.selfElo != 1900 || predictor.oppoElo != 2100 {
		t.Errorf("model was asked about %q with %d vs %d", predictor.position, predictor.selfElo, predictor.oppoElo)
	}
}

func TestPredictionsNeedAModel(t *testing.T) {
	query := url.Values{"selfElo": {"1900"}, "oppoElo": {"2100"}}

	body := decode[problem](t, get(t, sampleAPI(t), "/api/v1/predictions", query), http.StatusServiceUnavailable)

	if body.Detail == "" {
		t.Error("want a detail explaining that no model is loaded")
	}
}

func TestPredictionsRejectMissingOrImplausibleRatings(t *testing.T) {
	handler := sampleAPIFull(t, nil, &fakePredictor{})
	cases := map[string]url.Values{
		"no ratings":     {},
		"only one":       {"selfElo": {"1900"}},
		"not a number":   {"selfElo": {"strong"}, "oppoElo": {"2100"}},
		"far too low":    {"selfElo": {"5"}, "oppoElo": {"2100"}},
		"far too high":   {"selfElo": {"1900"}, "oppoElo": {"9000"}},
		"negative value": {"selfElo": {"-1900"}, "oppoElo": {"2100"}},
	}
	for name, query := range cases {
		recorder := get(t, handler, "/api/v1/predictions", query)
		if recorder.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, recorder.Code)
		}
	}
}

func TestPredictionsRejectAnUnreadablePosition(t *testing.T) {
	query := url.Values{"position": {"nonsense"}, "selfElo": {"1900"}, "oppoElo": {"2100"}}

	decode[problem](t, get(t, sampleAPIFull(t, nil, &fakePredictor{}), "/api/v1/predictions", query), http.StatusBadRequest)
}
