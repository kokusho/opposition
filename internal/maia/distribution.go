package maia

import (
	"math"
	"sort"

	"opposition/internal/board"
	"opposition/internal/core"
)

// distribution turns the model's raw scores into a probability for each
// legal move, most likely first. Scores of moves that are not legal are
// ignored, so the probabilities of the legal moves add up to 1.
func distribution(logits []float32, legal []board.Move, blackToMove bool) []core.PredictedMove {
	moves := make([]core.PredictedMove, 0, len(legal))
	scores := make([]float64, 0, len(legal))
	for _, move := range legal {
		seen := move.UCI
		if blackToMove {
			seen = mirrorMove(seen)
		}
		index, ok := moveIndex(seen)
		if !ok {
			continue
		}
		moves = append(moves, core.PredictedMove{SAN: move.SAN, UCI: move.UCI, After: move.After})
		scores = append(scores, float64(logits[index]))
	}

	for i, probability := range softmax(scores) {
		moves[i].Probability = probability
	}
	sort.SliceStable(moves, func(i, j int) bool { return moves[i].Probability > moves[j].Probability })
	return moves
}

// outcome reads the model's game-result prediction for the side to move. The
// model orders its three scores loss, draw, win.
func outcome(logits []float32) (win, draw, loss float64) {
	probabilities := softmax([]float64{float64(logits[0]), float64(logits[1]), float64(logits[2])})
	return probabilities[2], probabilities[1], probabilities[0]
}

func softmax(scores []float64) []float64 {
	if len(scores) == 0 {
		return nil
	}
	highest := scores[0]
	for _, score := range scores {
		highest = math.Max(highest, score)
	}
	probabilities := make([]float64, len(scores))
	var total float64
	for i, score := range scores {
		probabilities[i] = math.Exp(score - highest)
		total += probabilities[i]
	}
	for i := range probabilities {
		probabilities[i] /= total
	}
	return probabilities
}
