package core

import (
	"context"
	"sort"
)

// Evaluator judges a position, typically by asking a chess engine.
type Evaluator interface {
	Evaluate(ctx context.Context, position string) (Eval, error)
}

// Mistake is a habitual move of the opponent that an engine considers
// clearly worse than the best move.
type Mistake struct {
	Candidate
	// Loss is what the move gives away, in centipawns.
	Loss int
	// Before evaluates the position the move was played in, for the opponent.
	// Its Best is what the engine would have played instead.
	Before Eval
	// After evaluates the position after the move, for you. Its Line is how
	// the engine suggests to continue.
	After Eval
}

// FindMistakes evaluates each candidate and returns those that give away at
// least minLoss centipawns, ordered by how often the opponent plays the move
// times how much it costs. progress, if not nil, is called after each candidate.
func FindMistakes(ctx context.Context, candidates []Candidate, engine Evaluator, minLoss int, progress func(done, total int)) ([]Mistake, error) {
	var mistakes []Mistake
	for i, candidate := range candidates {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		before, err := engine.Evaluate(ctx, candidate.Position)
		if err != nil {
			return nil, err
		}
		after, err := engine.Evaluate(ctx, candidate.Move.After)
		if err != nil {
			return nil, err
		}
		if loss := Loss(before, after); loss >= minLoss {
			mistakes = append(mistakes, Mistake{Candidate: candidate, Loss: loss, Before: before, After: after})
		}
		if progress != nil {
			progress(i+1, len(candidates))
		}
	}
	sort.SliceStable(mistakes, func(i, j int) bool {
		return mistakes[i].severity() > mistakes[j].severity()
	})
	return mistakes, nil
}

// severity ranks a mistake by how reliably the opponent plays it times how
// much it costs them.
func (m Mistake) severity() float64 {
	return m.Move.Share * float64(m.Loss)
}
