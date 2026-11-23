package core

import (
	"sort"
	"strings"
	"time"
)

// Player is a name from the database with the number of games played under
// it and the rating from the most recent game that states one (0 if none).
type Player struct {
	Name   string
	Games  int
	Rating int
}

// Players lists everyone in the games whose name contains query (ignoring
// case), most games first. An empty query matches everyone.
func Players(games []Game, query string) []Player {
	type entry struct {
		games   int
		rating  int
		ratedOn time.Time
	}
	entries := map[string]*entry{}
	note := func(name string, rating int, date time.Time) {
		e := entries[name]
		if e == nil {
			e = &entry{}
			entries[name] = e
		}
		e.games++
		if rating == 0 {
			return
		}
		// A dated rating beats an undated one; among dated ones the latest wins.
		if e.rating == 0 || date.After(e.ratedOn) {
			e.rating, e.ratedOn = rating, date
		}
	}
	for _, game := range games {
		note(game.White, game.WhiteElo, game.Date)
		note(game.Black, game.BlackElo, game.Date)
	}

	query = strings.ToLower(query)
	players := make([]Player, 0, len(entries))
	for name, e := range entries {
		if !strings.Contains(strings.ToLower(name), query) {
			continue
		}
		players = append(players, Player{Name: name, Games: e.games, Rating: e.rating})
	}
	sort.Slice(players, func(i, j int) bool {
		if players[i].Games != players[j].Games {
			return players[i].Games > players[j].Games
		}
		return players[i].Name < players[j].Name
	})
	return players
}
