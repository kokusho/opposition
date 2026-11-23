// Package maia predicts human moves with the Maia-3 neural network, run
// through ONNX Runtime.
//
// The input encoding and the move vocabulary follow the official
// implementation (https://github.com/CSSLab/maia3, AGPL-3.0); the tests
// compare this package with predictions made by that implementation.
package maia

import "strings"

const (
	plainMoves     = 64 * 64 // every from-square to every to-square
	vocabularySize = plainMoves + 8*8*4
	promotions     = "qrbn"
)

// moveIndex returns the position of a move in the model's output. The move is
// in UCI notation as seen by the side to move with its own pieces at the
// bottom: Black's moves must be mirrored first.
//
// Plain moves come first, numbered from*64+to with squares counted a1, b1 ...
// h8. Promotions follow, always from the seventh to the eighth rank, ordered
// by from-file, to-file and piece (queen, rook, bishop, knight).
func moveIndex(uci string) (int, bool) {
	if len(uci) != 4 && len(uci) != 5 {
		return 0, false
	}
	fromFile, fromRank, ok := square(uci[0:2])
	if !ok {
		return 0, false
	}
	toFile, toRank, ok := square(uci[2:4])
	if !ok {
		return 0, false
	}

	if len(uci) == 4 {
		return (fromRank*8+fromFile)*64 + toRank*8 + toFile, true
	}

	piece := strings.IndexByte(promotions, uci[4])
	if piece < 0 || fromRank != 6 || toRank != 7 {
		return 0, false
	}
	return plainMoves + (fromFile*8+toFile)*4 + piece, true
}

// mirrorMove flips a move top to bottom, which turns a move of Black into
// the same move seen from Black's side of the board, and back.
func mirrorMove(uci string) string {
	flipped := []byte(uci)
	flipped[1] = '1' + '8' - uci[1]
	flipped[3] = '1' + '8' - uci[3]
	return string(flipped)
}

// square splits a square name such as "e4" into zero-based file and rank.
func square(name string) (file, rank int, ok bool) {
	file, rank = int(name[0]-'a'), int(name[1]-'1')
	return file, rank, file >= 0 && file < 8 && rank >= 0 && rank < 8
}
