package core

// evalCap is the largest advantage that is still told apart, in centipawns.
// Forced mates and anything beyond ten pawns all count as "winning".
const evalCap = 1000

// Eval is an engine's verdict on a position, from the point of view of the
// side to move.
type Eval struct {
	// Centipawns is the advantage of the side to move in hundredths of a
	// pawn. Unused when IsMate is set.
	Centipawns int
	// IsMate says the engine found a forced mate. MateIn is the number of
	// moves: positive if the side to move mates, zero or negative if it is
	// being mated.
	IsMate bool
	MateIn int
	// Best is the engine's preferred move in UCI notation, empty when the
	// game is over.
	Best string
	// Line is the continuation the engine expects, starting with Best.
	Line  []string
	Depth int
}

// Value is the advantage of the side to move in centipawns, capped at ten
// pawns either way.
func (e Eval) Value() int {
	if e.IsMate {
		if e.MateIn > 0 {
			return evalCap
		}
		return -evalCap
	}
	return max(-evalCap, min(evalCap, e.Centipawns))
}

// Loss is how many centipawns a move gives away: before is the position the
// move was played in, after the position it led to. An engine evaluates from
// the side to move, so after is seen through the other player's eyes.
func Loss(before, after Eval) int {
	return max(0, before.Value()+after.Value())
}
