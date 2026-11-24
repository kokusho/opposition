package core

import (
	"context"
	"sort"
)

// Rules knows the legal moves of a position, each with the position it
// leads to.
type Rules interface {
	Moves(position string) ([]Step, error)
}

// TrapOptions says what counts as a trap.
type TrapOptions struct {
	// TheirElo and MyElo are the ratings the move predictions are made for.
	TheirElo int
	MyElo    int
	// MinLoss is how many centipawns a reply must give away to count as
	// going wrong.
	MinLoss int
	// MinChance is how likely going wrong must be, 0 to 1.
	MinChance float64
	// MaxOwnLoss is how many centipawns worse than the engine's best move
	// your own move may be.
	MaxOwnLoss int
	// MaxReplies and MinReplyChance limit which predicted replies are
	// checked with the engine: at most that many, each at least that likely.
	MaxReplies     int
	MinReplyChance float64
}

// Reply is a predicted answer of the opponent with what it gives away.
type Reply struct {
	PredictedMove
	// Loss is in centipawns, compared with the opponent's best answer.
	Loss int
	// Eval is the engine's view of the position after the reply, where you
	// are to move.
	Eval Eval
}

// Trap is a sound move of yours after which a player of the opponent's
// rating is likely to go wrong.
type Trap struct {
	// Line leads from the start to Position, where you are to move.
	Line     []Step
	Position string
	// Games is how many of the opponent's games reached Position.
	Games int
	// Move is your move. FacedGames is how often the opponent has faced it;
	// zero means the engine suggested it.
	Move       Step
	FacedGames int
	// OwnLoss is what Move gives up compared with the engine's best move.
	OwnLoss int
	// After evaluates the position after Move, where the opponent is to move.
	After Eval
	// Replies are the opponent's likely answers, most likely first.
	Replies []Reply
	// ErrorChance is the summed probability of the replies that go wrong.
	ErrorChance float64
	// ExpectedLoss is the probability-weighted loss over the replies checked,
	// in centipawns. Replies that were not checked count as losing nothing.
	ExpectedLoss float64
}

type trapOption struct {
	move  Step
	faced int
}

// FindTraps looks, at each of your turns, at the moves the opponent has
// faced plus the engine's best move, and reports those after which the
// predicted replies go wrong often enough. Most costly traps come first.
// progress, if not nil, is called after each turn.
func FindTraps(ctx context.Context, turns []Turn, rules Rules, engine Evaluator, predictor Predictor,
	opts TrapOptions, progress func(done, total int)) ([]Trap, error) {
	var traps []Trap
	checked := map[string]bool{}
	for i, turn := range turns {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		before, err := engine.Evaluate(ctx, turn.Position)
		if err != nil {
			return nil, err
		}
		options, err := trapOptions(turn, before.Best, rules)
		if err != nil {
			return nil, err
		}
		for _, option := range options {
			if checked[option.move.Position] {
				continue
			}
			checked[option.move.Position] = true

			trap, found, err := judgeTrap(ctx, before, option, engine, predictor, opts)
			if err != nil {
				return nil, err
			}
			if found {
				trap.Line, trap.Position, trap.Games = turn.Line, turn.Position, turn.Games
				traps = append(traps, trap)
			}
		}
		if progress != nil {
			progress(i+1, len(turns))
		}
	}
	sort.SliceStable(traps, func(i, j int) bool { return traps[i].ExpectedLoss > traps[j].ExpectedLoss })
	return traps, nil
}

// trapOptions lists the moves worth trying at a turn: those the opponent has
// faced, and the engine's best move if it is not among them.
func trapOptions(turn Turn, best string, rules Rules) ([]trapOption, error) {
	options := make([]trapOption, 0, len(turn.Moves)+1)
	known := false
	for _, move := range turn.Moves {
		options = append(options, trapOption{
			move:  Step{SAN: move.SAN, UCI: move.UCI, Position: move.After},
			faced: move.Games,
		})
		known = known || move.UCI == best
	}
	if best == "" || known {
		return options, nil
	}

	legal, err := rules.Moves(turn.Position)
	if err != nil {
		return nil, err
	}
	for _, move := range legal {
		if move.UCI == best {
			options = append(options, trapOption{move: move})
		}
	}
	return options, nil
}

// judgeTrap checks one move of yours: is it sound, and how do the opponent's
// likely replies fare?
func judgeTrap(ctx context.Context, before Eval, option trapOption, engine Evaluator, predictor Predictor,
	opts TrapOptions) (Trap, bool, error) {
	after, err := engine.Evaluate(ctx, option.move.Position)
	if err != nil {
		return Trap{}, false, err
	}
	ownLoss := Loss(before, after)
	if ownLoss > opts.MaxOwnLoss {
		return Trap{}, false, nil
	}

	prediction, err := predictor.Predict(ctx, option.move.Position, opts.TheirElo, opts.MyElo)
	if err != nil {
		return Trap{}, false, err
	}
	trap := Trap{Move: option.move, FacedGames: option.faced, OwnLoss: ownLoss, After: after}
	for _, predicted := range prediction.Moves {
		if len(trap.Replies) >= opts.MaxReplies || predicted.Probability < opts.MinReplyChance {
			break
		}
		afterReply, err := engine.Evaluate(ctx, predicted.After)
		if err != nil {
			return Trap{}, false, err
		}
		loss := Loss(after, afterReply)
		trap.Replies = append(trap.Replies, Reply{PredictedMove: predicted, Loss: loss, Eval: afterReply})
		trap.ExpectedLoss += predicted.Probability * float64(loss)
		if loss >= opts.MinLoss {
			trap.ErrorChance += predicted.Probability
		}
	}
	return trap, len(trap.Replies) > 0 && trap.ErrorChance >= opts.MinChance, nil
}
