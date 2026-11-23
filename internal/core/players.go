package core

import (
	"sort"
	"strings"
)

// Player is a name from the database with the number of games played under it.
type Player struct {
	Name  string
	Games int
}

// Players lists everyone in the games whose name contains query (ignoring
// case), most games first. An empty query matches everyone.
func Players(games []Game, query string) []Player {
	counts := map[string]int{}
	for _, game := range games {
		counts[game.White]++
		counts[game.Black]++
	}

	query = strings.ToLower(query)
	players := make([]Player, 0, len(counts))
	for name, n := range counts {
		if !strings.Contains(strings.ToLower(name), query) {
			continue
		}
		players = append(players, Player{Name: name, Games: n})
	}
	sort.Slice(players, func(i, j int) bool {
		if players[i].Games != players[j].Games {
			return players[i].Games > players[j].Games
		}
		return players[i].Name < players[j].Name
	})
	return players
}
