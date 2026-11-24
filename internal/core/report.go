package core

import (
	"sort"
	"time"
)

// Summary describes the opponent's games with one colour.
type Summary struct {
	Games int
	// Wins, Draws and Losses are the opponent's results.
	Wins   int
	Draws  int
	Losses int
	// First and Last are the dates of the oldest and newest dated game; zero
	// if no game is dated.
	First time.Time
	Last  time.Time
}

// Summarize counts the games in which opponent had opponentColor.
func Summarize(games []Game, opponent string, opponentColor Color) Summary {
	var summary Summary
	for _, game := range games {
		if game.Player(opponentColor) != opponent {
			continue
		}
		summary.Games++
		switch game.Result {
		case Draw:
			summary.Draws++
		case wonBy(opponentColor):
			summary.Wins++
		case wonBy(opponentColor.Opposite()):
			summary.Losses++
		}
		if game.Date.IsZero() {
			continue
		}
		if summary.First.IsZero() || game.Date.Before(summary.First) {
			summary.First = game.Date
		}
		if game.Date.After(summary.Last) {
			summary.Last = game.Date
		}
	}
	return summary
}

// Line is a sequence of moves the opponent's games followed repeatedly.
type Line struct {
	Steps []Step
	// Games is how many games played the whole line. Wins, Draws and Losses
	// are the opponent's results in them.
	Games  int
	Wins   int
	Draws  int
	Losses int
}

// LineOptions limits which lines count as main lines.
type LineOptions struct {
	// MinGames is how many games must share a move for the line to go on.
	MinGames int
	// MaxPlies is the longest line in half-moves, MaxLines how many to keep.
	MaxPlies int
	MaxLines int
}

// MainLines returns the lines the opponent's games follow most often. A line
// runs for as long as at least opts.MinGames games stay together.
func (t *Tree) MainLines(opts LineOptions) []Line {
	var lines []Line
	var follow func(position string, steps []Step, last *MoveStat)
	follow = func(position string, steps []Step, last *MoveStat) {
		continued := false
		if len(steps) < opts.MaxPlies {
			for _, move := range t.Node(position).Moves {
				if move.Games < opts.MinGames {
					continue
				}
				continued = true
				next := append(steps[:len(steps):len(steps)], Step{SAN: move.SAN, UCI: move.UCI, Position: move.After})
				follow(move.After, next, &move)
			}
		}
		if !continued && last != nil {
			lines = append(lines, Line{
				Steps: steps, Games: last.Games, Wins: last.Wins, Draws: last.Draws, Losses: last.Losses,
			})
		}
	}
	follow(StartPosition, nil, nil)

	sort.SliceStable(lines, func(i, j int) bool { return lines[i].Games > lines[j].Games })
	if len(lines) > opts.MaxLines {
		lines = lines[:opts.MaxLines]
	}
	return lines
}

// WeakSpot is a move the opponent has faced and scored badly against.
type WeakSpot struct {
	// Line leads from the start to Position, where you are to move.
	Line     []Step
	Position string
	// Move is the move they faced, with their results against it.
	Move MoveStat
}

// WeakSpots lists the moves, at positions where the side with myColor is to
// move, that the opponent faced in at least opts.MinGames games and scored at
// most maxScore against (0 to 1), where they also faced another move to
// compare with. Most faced first, then worst score first.
func (t *Tree) WeakSpots(myColor Color, opts ScanOptions, maxScore float64) []WeakSpot {
	var spots []WeakSpot
	t.walk(opts.MaxPlies, func(position string, line []Step, node Node) {
		if SideToMove(position) != myColor {
			return
		}
		finishedHere := 0
		for _, move := range node.Moves {
			finishedHere += move.finished()
		}
		for _, move := range node.Moves {
			score, known := move.Score()
			// A bad score against the only move faced says nothing about that
			// move: it just repeats what an earlier move already showed.
			facedSomethingElse := move.finished() < finishedHere
			if move.Games >= opts.MinGames && known && score <= maxScore && facedSomethingElse {
				spots = append(spots, WeakSpot{Line: line, Position: position, Move: move})
			}
		}
	})
	sort.SliceStable(spots, func(i, j int) bool {
		if spots[i].Move.Games != spots[j].Move.Games {
			return spots[i].Move.Games > spots[j].Move.Games
		}
		a, _ := spots[i].Move.Score()
		b, _ := spots[j].Move.Score()
		return a < b
	})
	return spots
}
