// Package pgn reads PGN databases into the games the core works with.
package pgn

import (
	"io"
	"strings"
	"unicode/utf8"

	"github.com/corentings/chess/v2"

	"opposition/internal/core"
)

// Report says what happened while loading a PGN stream.
type Report struct {
	// Games is the number of games loaded.
	Games int
	// Skipped lists the games that could not be read.
	Skipped []Skipped
}

// Skipped is a game that was left out, with its 1-based number in the file.
type Skipped struct {
	Number int
	Reason string
}

// Load reads every game from a PGN stream. Games the parser rejects (for
// example because of an illegal move) are skipped and listed in the report.
// The error is only for failures reading the stream itself.
func Load(r io.Reader) ([]core.Game, Report, error) {
	var games []core.Game
	var report Report
	number := 0
	for raw, err := range splitGames(r) {
		if err != nil {
			return nil, Report{}, err
		}
		number++
		game, err := parseGame(toUTF8(raw))
		if err != nil {
			report.Skipped = append(report.Skipped, Skipped{Number: number, Reason: err.Error()})
			continue
		}
		games = append(games, game)
	}
	report.Games = len(games)
	return games, report, nil
}

func parseGame(raw string) (core.Game, error) {
	fromPGN, err := chess.PGN(strings.NewReader(raw))
	if err != nil {
		return core.Game{}, err
	}
	parsed := chess.NewGame(fromPGN)

	game := core.Game{
		White:  parsed.GetTagPair("White"),
		Black:  parsed.GetTagPair("Black"),
		Date:   parseDate(parsed.GetTagPair("Date")),
		Result: parseResult(parsed.GetTagPair("Result")),
	}

	moves := parsed.Moves()
	positions := parsed.Positions()
	game.Plies = make([]core.Ply, 0, len(moves))
	before := positionKey(positions[0])
	for i, move := range moves {
		after := positionKey(positions[i+1])
		game.Plies = append(game.Plies, core.Ply{
			Before: before,
			SAN:    chess.AlgebraicNotation{}.Encode(positions[i], move),
			UCI:    chess.UCINotation{}.Encode(positions[i], move),
			After:  after,
		})
		before = after
	}
	return game, nil
}

func parseResult(tag string) core.Result {
	switch tag {
	case "1-0":
		return core.WhiteWon
	case "0-1":
		return core.BlackWon
	case "1/2-1/2":
		return core.Draw
	}
	return core.ResultUnknown
}

// positionKey reduces a position to the first four X-FEN fields, dropping the
// move counters so that transpositions share a key.
func positionKey(pos *chess.Position) string {
	fields := strings.Fields(pos.XFENString())
	return strings.Join(fields[:4], " ")
}

// toUTF8 returns text unchanged if it is valid UTF-8 and otherwise reads it
// as Latin-1, the usual encoding of older PGN exports.
func toUTF8(text string) string {
	if utf8.ValidString(text) {
		return text
	}
	runes := make([]rune, len(text))
	for i := 0; i < len(text); i++ {
		runes[i] = rune(text[i])
	}
	return string(runes)
}
