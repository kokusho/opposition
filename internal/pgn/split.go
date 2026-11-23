package pgn

import (
	"bufio"
	"io"
	"iter"
	"strings"
)

const byteOrderMark = "\xef\xbb\xbf"

// splitGames yields the text of each game in a PGN stream. A game starts at a
// tag line ("[...") that follows move text, unless that line is inside a
// {brace comment}.
func splitGames(r io.Reader) iter.Seq2[string, error] {
	return func(yield func(string, error) bool) {
		reader := bufio.NewReader(r)
		var game strings.Builder
		inMoves := false
		inComment := false
		first := true

		for {
			line, err := reader.ReadString('\n')
			if err != nil && err != io.EOF {
				yield("", err)
				return
			}

			if first {
				line = strings.TrimPrefix(line, byteOrderMark)
				first = false
			}
			trimmed := strings.TrimSpace(line)
			isTag := !inComment && strings.HasPrefix(trimmed, "[")
			if isTag && inMoves {
				if !yield(strings.TrimSpace(game.String()), nil) {
					return
				}
				game.Reset()
				inMoves = false
			}
			if trimmed != "" && !isTag {
				inMoves = true
				inComment = endsInsideComment(trimmed, inComment)
			}
			game.WriteString(line)

			if err == io.EOF {
				break
			}
		}

		if last := strings.TrimSpace(game.String()); last != "" {
			yield(last, nil)
		}
	}
}

// endsInsideComment reports whether a brace comment is still open at the end
// of a line of move text. PGN brace comments do not nest.
func endsInsideComment(line string, inComment bool) bool {
	for _, ch := range line {
		switch ch {
		case '{':
			inComment = true
		case '}':
			inComment = false
		}
	}
	return inComment
}
