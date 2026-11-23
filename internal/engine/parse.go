package engine

import (
	"strconv"
	"strings"

	"opposition/internal/core"
)

// parseInfo reads one "info" line of UCI engine output. It reports false for
// lines without an exact score: progress messages and lower/upper bounds.
func parseInfo(line string) (core.Eval, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 || fields[0] != "info" {
		return core.Eval{}, false
	}

	var eval core.Eval
	scored := false
	for i := 1; i < len(fields); i++ {
		switch fields[i] {
		case "depth":
			if i+1 < len(fields) {
				eval.Depth, _ = strconv.Atoi(fields[i+1])
				i++
			}
		case "score":
			if i+2 >= len(fields) {
				return core.Eval{}, false
			}
			value, err := strconv.Atoi(fields[i+2])
			if err != nil {
				return core.Eval{}, false
			}
			switch fields[i+1] {
			case "cp":
				eval.Centipawns = value
			case "mate":
				eval.IsMate, eval.MateIn = true, value
			default:
				return core.Eval{}, false
			}
			scored = true
			i += 2
		case "lowerbound", "upperbound":
			return core.Eval{}, false
		case "pv":
			eval.Line = fields[i+1:]
			if len(eval.Line) > 0 {
				eval.Best = eval.Line[0]
			} else {
				eval.Line = nil
			}
			i = len(fields)
		}
	}
	return eval, scored
}
