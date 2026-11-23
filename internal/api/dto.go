package api

import "opposition/internal/core"

type nodeDTO struct {
	Position string `json:"position"`
	// SideToMove is "white" or "black".
	SideToMove string `json:"sideToMove"`
	// Mover says whose choice this position is: "you" or "opponent".
	Mover string    `json:"mover"`
	Games int       `json:"games"`
	Moves []moveDTO `json:"moves"`
}

type moveDTO struct {
	SAN string `json:"san"`
	UCI string `json:"uci"`
	// Position is the key of the position after the move.
	Position string  `json:"position"`
	Games    int     `json:"games"`
	Weight   float64 `json:"weight"`
	Share    float64 `json:"share"`
	// Wins, Draws, Losses and Score are the opponent's.
	Wins       int      `json:"wins"`
	Draws      int      `json:"draws"`
	Losses     int      `json:"losses"`
	Score      *float64 `json:"score,omitempty"`
	LastPlayed *string  `json:"lastPlayed,omitempty"`
}

type playerDTO struct {
	Name  string `json:"name"`
	Games int    `json:"games"`
	// Rating is from the player's most recent game that states one.
	Rating int `json:"rating,omitempty"`
}

func toNodeDTO(position string, myColor core.Color, node core.Node) nodeDTO {
	side := core.SideToMove(position)
	mover := "opponent"
	if side == myColor {
		mover = "you"
	}

	dto := nodeDTO{
		Position:   position,
		SideToMove: colorName(side),
		Mover:      mover,
		Games:      node.Games,
		Moves:      make([]moveDTO, 0, len(node.Moves)),
	}
	for _, move := range node.Moves {
		dto.Moves = append(dto.Moves, toMoveDTO(move))
	}
	return dto
}

func toMoveDTO(move core.MoveStat) moveDTO {
	dto := moveDTO{
		SAN:      move.SAN,
		UCI:      move.UCI,
		Position: move.After,
		Games:    move.Games,
		Weight:   move.Weight,
		Share:    move.Share,
		Wins:     move.Wins,
		Draws:    move.Draws,
		Losses:   move.Losses,
	}
	if score, ok := move.Score(); ok {
		dto.Score = &score
	}
	if !move.LastPlayed.IsZero() {
		date := move.LastPlayed.Format("2006-01-02")
		dto.LastPlayed = &date
	}
	return dto
}

func colorName(c core.Color) string {
	if c == core.Black {
		return "black"
	}
	return "white"
}
