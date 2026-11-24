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
	t.walk(opts.MaxPlies, func(position string, line []Step, node Node) {
		if SideToMove(position) != opponentColor {
			return
		}
		for _, move := range node.Moves {
			if move.Games >= opts.MinGames {
				candidates = append(candidates, Candidate{
					Line:      line,
					Position:  position,
					NodeGames: node.Games,
					Move:      move,
				})
			}
		}
	})
	return candidates
}

// Turn is a position inside the opponent's repertoire where it is your move.
type Turn struct {
	// Line leads from the start to Position.
	Line     []Step
	Position string
	// Games is the number of the opponent's games that reached Position.
	Games int
	// Moves is what the opponent has faced here.
	Moves []MoveStat
}

// MyTurns collects the positions where the side with myColor is to move and
// which the opponent reached in at least opts.MinGames games, most played
// lines first.
func (t *Tree) MyTurns(myColor Color, opts ScanOptions) []Turn {
	var turns []Turn
	t.walk(opts.MaxPlies, func(position string, line []Step, node Node) {
		if SideToMove(position) == myColor && node.Games >= opts.MinGames {
			turns = append(turns, Turn{Line: line, Position: position, Games: node.Games, Moves: node.Moves})
		}
	})
	return turns
}

// walk visits every position reachable from the start within maxPlies
// half-moves, each once, following the most played moves first. line is the
// route by which the position was first found.
func (t *Tree) walk(maxPlies int, visit func(position string, line []Step, node Node)) {
	visited := map[string]bool{}
	var step func(position string, line []Step)
	step = func(position string, line []Step) {
		if len(line) >= maxPlies || visited[position] {
			return
		}
		visited[position] = true
		node := t.Node(position)
		visit(position, line, node)
		for _, move := range node.Moves {
			next := append(line[:len(line):len(line)], Step{SAN: move.SAN, UCI: move.UCI, Position: move.After})
			step(move.After, next)
		}
	}
	step(StartPosition, nil)
}
