// Package core holds the opening-tree logic: which moves were played from
// which position in an opponent's games, and how those games went.
package core

import (
	"math"
	"sort"
	"time"
)

// Color is the side a player has in a game.
type Color int

const (
	White Color = iota
	Black
)

// Result is how a game ended.
type Result int

const (
	ResultUnknown Result = iota
	WhiteWon
	BlackWon
	Draw
)

// Opposite returns the other colour.
func (c Color) Opposite() Color {
	if c == White {
		return Black
	}
	return White
}

func wonBy(c Color) Result {
	if c == White {
		return WhiteWon
	}
	return BlackWon
}

// Ply is one half-move of a game. Before and After are position keys: any
// string that is equal for equal positions. The tree treats them as opaque.
type Ply struct {
	Before string
	SAN    string
	UCI    string
	After  string
}

// Game is one game from the database, reduced to what the tree needs.
type Game struct {
	White  string
	Black  string
	Date   time.Time
	Result Result
	Plies  []Ply
}

// Player returns the name of the player with the given colour.
func (g Game) Player(color Color) string {
	if color == White {
		return g.White
	}
	return g.Black
}

// TreeOptions tunes how games are counted. A game loses half its weight for
// every HalfLife that lies between its date and Now.
type TreeOptions struct {
	Now      time.Time
	HalfLife time.Duration
}

// weight discounts a game by its age. Undated games are treated as played on
// the fallback date.
func (o TreeOptions) weight(game Game, fallback time.Time) float64 {
	if o.HalfLife <= 0 {
		return 1
	}
	date := game.Date
	if date.IsZero() {
		date = fallback
	}
	if date.IsZero() {
		return 1
	}
	age := o.Now.Sub(date)
	return math.Pow(0.5, float64(age)/float64(o.HalfLife))
}

// MoveStat describes one move played from a position. Games is the raw number
// of games, Weight the same count with older games discounted. Wins, Draws and
// Losses are the opponent's results in the games that continued with this move.
type MoveStat struct {
	SAN    string
	UCI    string
	After  string
	Games  int
	Weight float64
	Share  float64
	Wins   int
	Draws  int
	Losses int
	// LastPlayed is the date of the most recent dated game with this move.
	LastPlayed time.Time
}

// Node is everything known about one position. Games is the number of games
// that reached it and continued with a move.
type Node struct {
	Games int
	Moves []MoveStat
}

// Tree maps positions to the moves played from them.
type Tree struct {
	nodes map[string]Node
}

// BuildTree collects the moves played in the games where opponent had
// opponentColor. A game counts once per position, by its first visit.
func BuildTree(games []Game, opponent string, opponentColor Color, opts TreeOptions) *Tree {
	relevant, oldest := gamesOf(games, opponent, opponentColor)

	// position key -> UCI move -> stats
	stats := map[string]map[string]*MoveStat{}
	for _, game := range relevant {
		weight := opts.weight(game, oldest)
		visited := map[string]bool{}
		for _, ply := range game.Plies {
			if visited[ply.Before] {
				continue
			}
			visited[ply.Before] = true

			moves := stats[ply.Before]
			if moves == nil {
				moves = map[string]*MoveStat{}
				stats[ply.Before] = moves
			}
			stat := moves[ply.UCI]
			if stat == nil {
				stat = &MoveStat{SAN: ply.SAN, UCI: ply.UCI, After: ply.After}
				moves[ply.UCI] = stat
			}
			stat.record(game, weight, opponentColor)
		}
	}

	tree := &Tree{nodes: make(map[string]Node, len(stats))}
	for position, moves := range stats {
		tree.nodes[position] = newNode(moves)
	}
	return tree
}

// gamesOf returns the games in which player had the given colour, and the
// date of the oldest dated one (zero if none is dated).
func gamesOf(games []Game, player string, color Color) (selected []Game, oldest time.Time) {
	for _, game := range games {
		if game.Player(color) != player {
			continue
		}
		selected = append(selected, game)
		if !game.Date.IsZero() && (oldest.IsZero() || game.Date.Before(oldest)) {
			oldest = game.Date
		}
	}
	return selected, oldest
}

func (s *MoveStat) record(game Game, weight float64, opponentColor Color) {
	s.Games++
	s.Weight += weight
	if game.Date.After(s.LastPlayed) {
		s.LastPlayed = game.Date
	}
	switch game.Result {
	case Draw:
		s.Draws++
	case wonBy(opponentColor):
		s.Wins++
	case wonBy(opponentColor.Opposite()):
		s.Losses++
	}
}

// newNode turns the per-move stats of one position into a node with shares
// filled in and the heaviest move first.
func newNode(moves map[string]*MoveStat) Node {
	node := Node{Moves: make([]MoveStat, 0, len(moves))}
	var total float64
	for _, stat := range moves {
		node.Games += stat.Games
		total += stat.Weight
	}
	for _, stat := range moves {
		stat.Share = stat.Weight / total
		node.Moves = append(node.Moves, *stat)
	}
	sort.Slice(node.Moves, func(i, j int) bool {
		a, b := node.Moves[i], node.Moves[j]
		if a.Weight != b.Weight {
			return a.Weight > b.Weight
		}
		return a.SAN < b.SAN
	})
	return node
}

// Node returns what is known about the position with the given key.
func (t *Tree) Node(position string) Node {
	return t.nodes[position]
}

// StartPosition is the position key of the initial chess position. Keys are
// the first four FEN fields (pieces, side to move, castling, en passant),
// with the en-passant square set only when a capture is possible.
const StartPosition = "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq -"

// Score is the opponent's score in the finished games with this move: one
// point per win and half a point per draw, divided by the number of finished
// games. It is unknown (false) when no game with this move has a result.
func (s MoveStat) Score() (float64, bool) {
	finished := s.Wins + s.Draws + s.Losses
	if finished == 0 {
		return 0, false
	}
	return (float64(s.Wins) + 0.5*float64(s.Draws)) / float64(finished), true
}
