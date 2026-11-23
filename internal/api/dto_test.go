package api

import (
	"encoding/json"
	"testing"
	"time"

	"opposition/internal/core"
)

const afterE4 = "rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -"

func TestNodeDTOSaysWhoseChoiceThePositionIs(t *testing.T) {
	cases := []struct {
		position string
		myColor  core.Color
		side     string
		mover    string
	}{
		{core.StartPosition, core.White, "white", "you"},
		{core.StartPosition, core.Black, "white", "opponent"},
		{afterE4, core.White, "black", "opponent"},
		{afterE4, core.Black, "black", "you"},
	}
	for _, c := range cases {
		dto := toNodeDTO(c.position, c.myColor, core.Node{})
		if dto.SideToMove != c.side || dto.Mover != c.mover {
			t.Errorf("position %q as %v: side %q mover %q, want %q %q",
				c.position, c.myColor, dto.SideToMove, dto.Mover, c.side, c.mover)
		}
	}
}

func TestNodeDTOCarriesEveryMoveStatistic(t *testing.T) {
	node := core.Node{Games: 4, Moves: []core.MoveStat{{
		SAN: "c5", UCI: "c7c5", After: "next", Games: 4, Weight: 2.5, Share: 1,
		Wins: 2, Draws: 1, Losses: 1,
		LastPlayed: time.Date(2026, time.March, 15, 0, 0, 0, 0, time.UTC),
	}}}

	dto := toNodeDTO(afterE4, core.White, node)

	got, _ := json.Marshal(dto)
	want := `{"position":"rnbqkbnr/pppppppp/8/8/4P3/8/PPPP1PPP/RNBQKBNR b KQkq -","sideToMove":"black","mover":"opponent","games":4,` +
		`"moves":[{"san":"c5","uci":"c7c5","position":"next","games":4,"weight":2.5,"share":1,` +
		`"wins":2,"draws":1,"losses":1,"score":0.625,"lastPlayed":"2026-03-15"}]}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

func TestNodeDTOLeavesOutScoreAndDateWhenUnknown(t *testing.T) {
	node := core.Node{Games: 1, Moves: []core.MoveStat{{SAN: "c5", UCI: "c7c5", After: "next", Games: 1, Weight: 1, Share: 1}}}

	move := toNodeDTO(afterE4, core.White, node).Moves[0]

	if move.Score != nil || move.LastPlayed != nil {
		t.Errorf("score = %v, lastPlayed = %v; want both nil", move.Score, move.LastPlayed)
	}
}

func TestNodeDTOForAnUnknownPositionHasAnEmptyMoveList(t *testing.T) {
	got, _ := json.Marshal(toNodeDTO("never seen w - -", core.White, core.Node{}))

	want := `{"position":"never seen w - -","sideToMove":"white","mover":"you","games":0,"moves":[]}`
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
