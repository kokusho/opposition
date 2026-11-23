package core

import "strings"

// Step is one move along a line, with the key of the position it leads to.
type Step struct {
	SAN      string
	UCI      string
	Position string
}

// Candidate is a move the opponent has repeatedly played in a position where
// it was their turn: something to check with an engine.
type Candidate struct {
	// Line leads from the start to Position.
	Line     []Step
	Position string
	// NodeGames is the number of the opponent's games that reached Position.
	NodeGames int
	Move      MoveStat
}

// ScanOptions limits which of the opponent's moves count as habits.
type ScanOptions struct {
	// MinGames is how often a move must have been played.
	MinGames int
	// MaxPlies is how many half-moves deep the scan looks.
	MaxPlies int
}

// SideToMove reads the side to move from a position key (its second field).
func SideToMove(position string) Color {
	fields := strings.Fields(position)
	if len(fields) > 1 && fields[1] == "b" {
		return Black
	}
	return White
}

// Candidates walks the tree from the start position and collects the
// opponent's habitual moves, most played lines first. A position that several
// move orders lead to is reported once, by the first route found.
func (t *Tree) Candidates(opponentColor Color, opts ScanOptions) []Candidate {
	var candidates []Candidate
	visited := map[string]bool{}
	var walk func(position string, line []Step)
	walk = func(position string, line []Step) {
		if len(line) >= opts.MaxPlies || visited[position] {
			return
		}
		visited[position] = true
		node := t.Node(position)
		for _, move := range node.Moves {
			if SideToMove(position) == opponentColor && move.Games >= opts.MinGames {
				candidates = append(candidates, Candidate{
					Line:      line,
					Position:  position,
					NodeGames: node.Games,
					Move:      move,
				})
			}
			next := append(line[:len(line):len(line)], Step{SAN: move.SAN, UCI: move.UCI, Position: move.After})
			walk(move.After, next)
		}
	}
	walk(StartPosition, nil)
	return candidates
}
