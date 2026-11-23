package maia

import (
	"context"
	"errors"
	"fmt"
	"os"
	"runtime"
	"sync"

	ort "github.com/yalue/onnxruntime_go"

	"opposition/internal/board"
	"opposition/internal/core"
)

// ErrNoRuntime means the ONNX Runtime shared library was not found.
var ErrNoRuntime = errors.New("ONNX Runtime library not found")

// runtimeEnv names an environment variable that overrides where the ONNX
// Runtime shared library is looked for.
const runtimeEnv = "ONNXRUNTIME_LIB"

var (
	inputNames  = []string{"tokens", "elo_self", "elo_oppo"}
	outputNames = []string{"logits_move", "logits_value"}

	runtimeOnce sync.Once
	runtimeErr  error
)

// Model is a loaded Maia-3 network.
type Model struct {
	session *ort.DynamicAdvancedSession
}

// Load opens a Maia-3 ONNX model file. libraryPath is the ONNX Runtime shared
// library; when empty, the usual install locations are searched.
func Load(modelPath, libraryPath string) (*Model, error) {
	if err := startRuntime(libraryPath); err != nil {
		return nil, err
	}
	if _, err := os.Stat(modelPath); err != nil {
		return nil, fmt.Errorf("maia model: %w", err)
	}
	session, err := ort.NewDynamicAdvancedSession(modelPath, inputNames, outputNames, nil)
	if err != nil {
		return nil, fmt.Errorf("maia model %s: %w", modelPath, err)
	}
	return &Model{session: session}, nil
}

// startRuntime loads ONNX Runtime once per process.
func startRuntime(libraryPath string) error {
	runtimeOnce.Do(func() {
		if libraryPath == "" {
			libraryPath = findRuntime()
		}
		if libraryPath == "" {
			runtimeErr = ErrNoRuntime
			return
		}
		ort.SetSharedLibraryPath(libraryPath)
		if err := ort.InitializeEnvironment(); err != nil {
			runtimeErr = fmt.Errorf("starting ONNX Runtime from %s: %w", libraryPath, err)
		}
	})
	return runtimeErr
}

// findRuntime looks for the ONNX Runtime shared library where package
// managers put it.
func findRuntime() string {
	candidates := []string{os.Getenv(runtimeEnv)}
	switch runtime.GOOS {
	case "darwin":
		candidates = append(candidates,
			"/opt/homebrew/lib/libonnxruntime.dylib",
			"/usr/local/lib/libonnxruntime.dylib")
	case "windows":
		candidates = append(candidates, "onnxruntime.dll")
	default:
		candidates = append(candidates,
			"/usr/lib/libonnxruntime.so",
			"/usr/local/lib/libonnxruntime.so",
			"/usr/lib/x86_64-linux-gnu/libonnxruntime.so")
	}
	for _, path := range candidates {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}

// Predict gives every legal move in the position the probability that a
// player rated selfElo plays it against a player rated oppoElo. Ratings are
// on the scale the model was trained on, Lichess blitz. The model sees only
// the current position, not the moves that led to it.
func (m *Model) Predict(ctx context.Context, position string, selfElo, oppoElo int) (core.Prediction, error) {
	if err := ctx.Err(); err != nil {
		return core.Prediction{}, err
	}
	parsed, err := board.Parse(position)
	if err != nil {
		return core.Prediction{}, err
	}
	legal, err := board.LegalMoves(position)
	if err != nil {
		return core.Prediction{}, err
	}
	if len(legal) == 0 {
		return core.Prediction{}, nil
	}

	tokens, err := ort.NewTensor(ort.NewShape(1, 64, channels), encodePosition(parsed))
	if err != nil {
		return core.Prediction{}, err
	}
	defer tokens.Destroy()
	self, err := ort.NewTensor(ort.NewShape(1), []float32{float32(selfElo)})
	if err != nil {
		return core.Prediction{}, err
	}
	defer self.Destroy()
	oppo, err := ort.NewTensor(ort.NewShape(1), []float32{float32(oppoElo)})
	if err != nil {
		return core.Prediction{}, err
	}
	defer oppo.Destroy()

	// Nil outputs are allocated by the runtime during the run.
	outputs := []ort.Value{nil, nil}
	if err := m.session.Run([]ort.Value{tokens, self, oppo}, outputs); err != nil {
		return core.Prediction{}, fmt.Errorf("running maia model: %w", err)
	}
	defer func() {
		for _, output := range outputs {
			if output != nil {
				_ = output.Destroy()
			}
		}
	}()

	moveLogits, ok := outputs[0].(*ort.Tensor[float32])
	if !ok || len(moveLogits.GetData()) != vocabularySize {
		return core.Prediction{}, errors.New("maia model: unexpected move output; is this a Maia-3 model?")
	}
	valueLogits, ok := outputs[1].(*ort.Tensor[float32])
	if !ok || len(valueLogits.GetData()) != 3 {
		return core.Prediction{}, errors.New("maia model: unexpected value output; is this a Maia-3 model?")
	}

	prediction := core.Prediction{
		Moves: distribution(moveLogits.GetData(), legal, core.SideToMove(position) == core.Black),
	}
	prediction.Win, prediction.Draw, prediction.Loss = outcome(valueLogits.GetData())
	return prediction, nil
}

// Close releases the model.
func (m *Model) Close() error {
	return m.session.Destroy()
}
