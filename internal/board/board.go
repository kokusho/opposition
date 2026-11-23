// Package board answers questions about a single chess position: its key,
// its legal moves and where they lead.
package board

import (
	"fmt"
	"sort"
	"strings"

	"github.com/corentings/chess/v2"
)

// Move is a legal move with the key of the position it leads to.
type Move struct {
	UCI   string
	SAN   string
	After string
}

// Key reduces a position to the first four X-FEN fields (pieces, side to
// move, castling, en passant only if a capture is possible). Dropping the
// move counters makes transpositions share a key.
func Key(position *chess.Position) string {
	fields := strings.Fields(position.XFENString())
	return strings.Join(fields[:4], " ")
}

// Parse turns a position key back into a position.
func Parse(key string) (*chess.Position, error) {
	fromFEN, err := chess.FEN(key + " 0 1")
	if err != nil {
		return nil, fmt.Errorf("position %q: %w", key, err)
	}
	return chess.NewGame(fromFEN).Position(), nil
}

// LegalMoves lists the legal moves in the position with the given key,
// ordered by UCI notation. It is empty when the game is over.
func LegalMoves(key string) ([]Move, error) {
	position, err := Parse(key)
	if err != nil {
		return nil, err
	}

	legal := position.ValidMoves()
	moves := make([]Move, 0, len(legal))
	for i := range legal {
		move := &legal[i]
		moves = append(moves, Move{
			UCI:   chess.UCINotation{}.Encode(position, move),
			SAN:   chess.AlgebraicNotation{}.Encode(position, move),
			After: Key(position.Update(move)),
		})
	}
	sort.Slice(moves, func(i, j int) bool { return moves[i].UCI < moves[j].UCI })
	return moves, nil
}
