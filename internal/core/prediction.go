package core

import "context"

// PredictedMove is a legal move with the probability that a human of the
// given rating plays it.
type PredictedMove struct {
	SAN         string
	UCI         string
	After       string
	Probability float64
}

// Prediction is a model's guess at what a human does in a position.
type Prediction struct {
	// Moves holds every legal move, most likely first.
	Moves []PredictedMove
	// Win, Draw and Loss predict how the game ends for the side to move
	// between players of the given ratings. They add up to 1.
	Win  float64
	Draw float64
	Loss float64
}

// Predictor guesses human moves. selfElo is the rating of the side to move,
// oppoElo that of the other player.
type Predictor interface {
	Predict(ctx context.Context, position string, selfElo, oppoElo int) (Prediction, error)
}
