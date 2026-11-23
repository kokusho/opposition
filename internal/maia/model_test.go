package maia

import (
	"context"
	"errors"
	"math"
	"os"
	"sort"
	"testing"
)

const modelFile = "../../models/maia3-5m.fp32.onnx"

// sizes are the released model sizes, smallest first.
var sizes = []string{"5m", "23m", "79m"}

// loadModel opens the 5M model, skipping the test when the model file or
// ONNX Runtime is not installed on this machine.
func loadModel(t testing.TB) *Model {
	t.Helper()
	return loadModelOf(t, "5m")
}

// loadModelOf opens the model of one size, skipping when it is not there.
func loadModelOf(t testing.TB, size string) *Model {
	t.Helper()
	path := "../../models/maia3-" + size + ".fp32.onnx"
	if _, err := os.Stat(path); err != nil {
		t.Skipf("model file %s is not present", path)
	}
	model, err := Load(path, "")
	if errors.Is(err, ErrNoRuntime) {
		t.Skip("ONNX Runtime is not installed")
	}
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	t.Cleanup(func() { _ = model.Close() })
	return model
}

func TestPredictMatchesTheOfficialModel(t *testing.T) {
	for _, size := range sizes {
		t.Run(size, func(t *testing.T) { matchesTheOfficialModel(t, size) })
	}
}

func matchesTheOfficialModel(t *testing.T, size string) {
	model := loadModelOf(t, size)
	const tolerance = 1e-4

	for _, c := range loadReferenceOf(t, size) {
		prediction, err := model.Predict(context.Background(), c.position(), c.SelfElo, c.OppoElo)
		if err != nil {
			t.Errorf("%s: %v", c.Name, err)
			continue
		}

		if len(prediction.Moves) != len(c.Moves) {
			t.Errorf("%s: %d moves, want %d", c.Name, len(prediction.Moves), len(c.Moves))
			continue
		}
		for _, move := range prediction.Moves {
			want, ok := c.Moves[move.UCI]
			if !ok || math.Abs(move.Probability-want) > tolerance {
				t.Errorf("%s: %s = %.5f, official %.5f (known=%v)", c.Name, move.UCI, move.Probability, want, ok)
			}
		}
		if math.Abs(prediction.Win-c.Win) > tolerance || math.Abs(prediction.Draw-c.Draw) > tolerance ||
			math.Abs(prediction.Loss-c.Loss) > tolerance {
			t.Errorf("%s: win/draw/loss %.4f/%.4f/%.4f, official %.4f/%.4f/%.4f",
				c.Name, prediction.Win, prediction.Draw, prediction.Loss, c.Win, c.Draw, c.Loss)
		}
		mostLikelyFirst := sort.SliceIsSorted(prediction.Moves, func(i, j int) bool {
			return prediction.Moves[i].Probability > prediction.Moves[j].Probability
		})
		if !mostLikelyFirst {
			t.Errorf("%s: moves are not ordered by probability", c.Name)
		}
	}
}

func TestPredictForAFinishedGameHasNoMoves(t *testing.T) {
	model := loadModel(t)

	// Fool's mate: White is checkmated.
	prediction, err := model.Predict(context.Background(), "rnb1kbnr/pppp1ppp/8/4p3/6Pq/5P2/PPPPP2P/RNBQKBNR w KQkq -", 1500, 1500)

	if err != nil || len(prediction.Moves) != 0 {
		t.Errorf("got %d moves (err %v), want none and no error", len(prediction.Moves), err)
	}
}

func TestPredictRejectsAnUnreadablePosition(t *testing.T) {
	model := loadModel(t)

	if _, err := model.Predict(context.Background(), "not a position", 1500, 1500); err == nil {
		t.Error("want an error")
	}
}

func TestLoadFailsForAMissingModelFile(t *testing.T) {
	loadModel(t) // skips where the runtime is missing, so the error below is about the file

	if _, err := Load("testdata/no-such-model.onnx", ""); err == nil {
		t.Error("want an error for a missing model file")
	}
}

// BenchmarkPredict times one prediction per model size from a middlegame.
func BenchmarkPredict(b *testing.B) {
	const middlegame = "r2q1rk1/pp2bppp/2n1bn2/3p4/3N4/2N1B3/PPPQBPPP/R4RK1 b - -"
	for _, size := range sizes {
		b.Run(size, func(b *testing.B) {
			model := loadModelOf(b, size)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				if _, err := model.Predict(context.Background(), middlegame, 2000, 2000); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}
