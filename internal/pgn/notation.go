package pgn

import (
	"fmt"

	"github.com/corentings/chess/v2"
)

// SANLine converts a line of UCI moves ("g1f3"), as engines report them, into
// standard notation ("Nf3"), playing them from the position with the given key.
func SANLine(position string, uci []string) ([]string, error) {
	fromFEN, err := chess.FEN(position + " 0 1")
	if err != nil {
		return nil, fmt.Errorf("position %q: %w", position, err)
	}
	game := chess.NewGame(fromFEN)

	sans := make([]string, 0, len(uci))
	for _, text := range uci {
		current := game.Position()
		move, err := chess.UCINotation{}.Decode(current, text)
		if err != nil {
			return nil, fmt.Errorf("move %q: %w", text, err)
		}
		san := chess.AlgebraicNotation{}.Encode(current, move)
		if err := game.Move(move, nil); err != nil {
			return nil, fmt.Errorf("move %q: %w", text, err)
		}
		sans = append(sans, san)
	}
	return sans, nil
}
