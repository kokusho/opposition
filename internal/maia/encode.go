package maia

import (
	"github.com/corentings/chess/v2"

	"opposition/internal/board"
)

const (
	channels  = 12 // six piece types for the mover, six for the opponent
	tokenSize = 64 * channels
)

// encode turns a position key into the model's input: for each of the 64
// squares, twelve numbers of which one is 1 if a piece stands there.
func encode(key string) ([]float32, error) {
	position, err := board.Parse(key)
	if err != nil {
		return nil, err
	}
	return encodePosition(position), nil
}

// encodePosition shows the board from the side to move: its pieces fill the
// first six channels (pawn, knight, bishop, rook, queen, king) and the board
// is flipped top to bottom when Black is to move, so the mover always plays
// up the board. Castling rights and en passant are not part of the input.
func encodePosition(position *chess.Position) []float32 {
	tokens := make([]float32, tokenSize)
	mover := position.Turn()
	for at, piece := range position.Board().SquareMap() {
		channel := pieceChannel(piece.Type())
		if channel < 0 {
			continue
		}
		if piece.Color() != mover {
			channel += 6
		}
		file, rank := int(at)%8, int(at)/8
		if mover == chess.Black {
			rank = 7 - rank
		}
		tokens[(rank*8+file)*channels+channel] = 1
	}
	return tokens
}

func pieceChannel(kind chess.PieceType) int {
	switch kind {
	case chess.Pawn:
		return 0
	case chess.Knight:
		return 1
	case chess.Bishop:
		return 2
	case chess.Rook:
		return 3
	case chess.Queen:
		return 4
	case chess.King:
		return 5
	}
	return -1
}
