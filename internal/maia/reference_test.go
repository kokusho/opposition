package maia

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// referenceCase is one position evaluated by the official Maia-3
// implementation; see tools/maia_reference.py.
type referenceCase struct {
	Name    string             `json:"name"`
	FEN     string             `json:"fen"`
	SelfElo int                `json:"selfElo"`
	OppoElo int                `json:"oppoElo"`
	Moves   map[string]float64 `json:"moves"`
	Win     float64            `json:"win"`
	Draw    float64            `json:"draw"`
	Loss    float64            `json:"loss"`
	Tokens  []int              `json:"tokens"`
}

// position is the case's FEN as a position key: without the move counters.
func (c referenceCase) position() string {
	return strings.Join(strings.Fields(c.FEN)[:4], " ")
}

// loadReference reads the official predictions of the 5M model.
func loadReference(t *testing.T) []referenceCase {
	t.Helper()
	return loadReferenceOf(t, "5m")
}

// loadReferenceOf reads the official predictions of one model size.
func loadReferenceOf(t *testing.T, size string) []referenceCase {
	t.Helper()
	data, err := os.ReadFile("../../testdata/maia/reference-" + size + ".json")
	if err != nil {
		t.Fatal(err)
	}
	var file struct {
		Cases []referenceCase `json:"cases"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		t.Fatal(err)
	}
	if len(file.Cases) == 0 {
		t.Fatal("reference file has no cases")
	}
	return file.Cases
}
