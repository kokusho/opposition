package api

import (
	"fmt"
	"strings"
)

// numbered writes moves with move numbers: "1. e4 c5 2. Nf3", or
// "2... d5 3. exd5" when the first move is Black's. firstPly counts
// half-moves from the start of the game, so 0 is White's first move.
func numbered(sans []string, firstPly int) string {
	var text strings.Builder
	for i, san := range sans {
		ply := firstPly + i
		if i > 0 {
			text.WriteByte(' ')
		}
		switch {
		case ply%2 == 0:
			fmt.Fprintf(&text, "%d. ", ply/2+1)
		case i == 0:
			fmt.Fprintf(&text, "%d... ", ply/2+1)
		}
		text.WriteString(san)
	}
	return text.String()
}

func sansOf(line []stepDTO) []string {
	sans := make([]string, 0, len(line))
	for _, step := range line {
		sans = append(sans, step.SAN)
	}
	return sans
}

// after introduces a move by the line leading to it: "After 1. e4 c5: " or
// "From the start: " for an empty line.
func after(line []stepDTO) string {
	if len(line) == 0 {
		return "From the start: "
	}
	return "After " + numbered(sansOf(line), 0) + ": "
}

func pawns(centipawns int) string {
	return fmt.Sprintf("%.1f", float64(centipawns)/100)
}

func percent(fraction float64) string {
	return fmt.Sprintf("%.0f%%", fraction*100)
}

func results(wins, draws, losses int) string {
	return fmt.Sprintf("+%d =%d -%d", wins, draws, losses)
}

func capitalised(color string) string {
	return strings.ToUpper(color[:1]) + color[1:]
}

// renderFacts writes a report as plain text for a language model to put into
// prose. Everything the model may say must be in here, and what was not
// analysed is said so explicitly, so that nothing gets made up to fill a gap.
func renderFacts(report reportDTO) string {
	var text strings.Builder
	line := func(format string, args ...any) { fmt.Fprintf(&text, format+"\n", args...) }

	theirColor := "black"
	if report.MyColor == "black" {
		theirColor = "white"
	}
	line("Opponent: %s (rating used: %d). You play %s (rating used: %d); they play %s.",
		report.Opponent, report.TheirElo, capitalised(report.MyColor), report.MyElo, capitalised(theirColor))

	summary := report.Summary
	line("Database: %d games of theirs as %s. Their results: %s.",
		summary.Games, capitalised(theirColor), results(summary.Wins, summary.Draws, summary.Losses))
	if summary.First != "" {
		line("Their dated games run from %s to %s.", summary.First, summary.Last)
	}

	line("")
	if len(report.MainLines) == 0 {
		line("Main lines: none; no line was played more than once.")
	} else {
		line("Main lines they follow (their results in brackets):")
		for _, main := range report.MainLines {
			line("- %s (%d games, %s)", numbered(sansOf(main.Line), 0), main.Games, results(main.Wins, main.Draws, main.Losses))
		}
	}

	line("")
	if len(report.WeakSpots) == 0 {
		line("Moves they score badly against: none stand out.")
	} else {
		line("Moves they score badly against:")
		for _, spot := range report.WeakSpots {
			score := "unknown"
			if spot.Move.Score != nil {
				score = percent(*spot.Move.Score)
			}
			line("- %s%s (%d games, they scored %s, %s)", after(spot.Line), numbered([]string{spot.Move.SAN}, len(spot.Line)),
				spot.Move.Games, score, results(spot.Move.Wins, spot.Move.Draws, spot.Move.Losses))
		}
	}

	line("")
	switch {
	case report.Mistakes == nil:
		line("Recurring mistakes: not analysed (no chess engine available).")
	case len(*report.Mistakes) == 0:
		line("Recurring mistakes: none found. No move they repeated loses half a pawn or more according to the engine.")
	default:
		line("Recurring mistakes (moves they repeated that the engine dislikes):")
		for _, mistake := range *report.Mistakes {
			ply := len(mistake.Line)
			text.WriteString("- " + numbered(append(sansOf(mistake.Line), mistake.Move.SAN), 0))
			fmt.Fprintf(&text, ": played in %d of %d games, gives away %s pawns", mistake.Move.Games, mistake.NodeGames, pawns(mistake.Loss))
			if mistake.Best != "" {
				text.WriteString("; the engine prefers " + numbered([]string{mistake.Best}, ply))
			}
			if len(mistake.Refutation) > 0 {
				text.WriteString("; you continue " + numbered(mistake.Refutation, ply+1))
			}
			line(" (%s for you)", mistake.Eval.Text)
		}
	}

	line("")
	switch {
	case report.Traps == nil:
		line("Traps: not analysed (needs both a chess engine and a move-prediction model).")
	case len(*report.Traps) == 0:
		line("Traps: none found. No sound move of yours makes a player of their rating go wrong often enough.")
	default:
		line("Traps (sound moves of yours after which a player of their rating often goes wrong; the percentages are a model's estimate for the rating, not this person's record):")
		for _, trap := range *report.Traps {
			ply := len(trap.Line)
			faced := "they never faced it in the database"
			if trap.FacedGames > 0 {
				faced = fmt.Sprintf("they faced it %d times", trap.FacedGames)
			}
			line("- %s%s (%s for you; %s). A %d-rated player goes wrong %s of the time:", after(trap.Line),
				numbered([]string{trap.Move.SAN}, ply), trap.Eval.Text, faced, report.TheirElo, percent(trap.ErrorChance))
			for _, reply := range trap.Replies {
				move := numbered([]string{reply.SAN}, ply+1)
				if reply.Loss == 0 {
					line("  - %s (%s) is fine (%s for you)", move, percent(reply.Probability), reply.Eval.Text)
					continue
				}
				answer := ""
				if reply.Punish != "" {
					answer = ", answer " + numbered([]string{reply.Punish}, ply+2)
				}
				line("  - %s (%s) loses %s pawns%s (%s for you)", move, percent(reply.Probability), pawns(reply.Loss), answer, reply.Eval.Text)
			}
		}
	}
	return text.String()
}
