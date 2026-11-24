# opposition

Prepare for a chess game against one specific opponent. Point it at a PGN
database, say who you are playing and with which colour, and it shows what
they play, where they go wrong, and which of your moves are most likely to
make them go wrong.

It cannot tell you how to win. It tells you where this opponent is
predictable and where players like them make mistakes, and it shows how much
evidence each claim rests on.

Status: release candidate 1. It has been tested against small hand-written
databases, not yet against a large real one.

## What you get

**Explore.** A board and the opponent's opening tree: every move they played
in a position, how often (older games count less), how they scored, and when
they last played it. Positions reached by different move orders are merged.
You can play any legal move on the board, including ones outside their games.

**Engine.** With Stockfish available, each move gets an evaluation from your
point of view, and moves that give away half a pawn or more are marked.

**Predictions.** With a Maia-3 model available, the opponent's turns show
what a player of their rating usually plays, with a probability per move.
This keeps working where their games run out.

**Prep report.** One page that collects:

- *Traps*: sound moves of yours after which a player of their rating often
  goes wrong, with their likely replies and your answer to each.
- *Recurring mistakes*: moves they played at least twice that the engine
  considers clearly inferior.
- *Moves they score badly against*: from their results alone.
- *Main lines*: what their games follow.
- *Written briefing* (optional): a language model puts the findings into
  prose.

Every finding opens on the board when clicked.

## Requirements

| What | Needed for | Install (macOS) |
|---|---|---|
| Go 1.26+, a recent Node (developed with 26), a C compiler | building | Xcode Command Line Tools, Homebrew |
| Stockfish | evaluations, mistakes, traps | `brew install stockfish` |
| ONNX Runtime | predictions, traps | `brew install onnxruntime` |
| A Maia-3 model file | predictions, traps | `make models` |
| An OpenAI-compatible API | written briefing | your choice |

Only the first row is required. Without Stockfish, ONNX Runtime or a model,
the server starts anyway and the features that need them are switched off;
the start-up log says which.

## Running

```sh
make models                           # once: downloads the 79M model, 312 MB
make run PGN=/path/to/database.pgn    # builds, then serves http://localhost:8080
```

The log reports how many games were loaded and which were skipped as
unreadable.

Flags of `bin/opposition`:

| Flag | Default | Meaning |
|---|---|---|
| `-pgn` | (required) | the PGN database |
| `-addr` | `localhost:8080` | where to listen |
| `-engine` | `stockfish` | UCI engine to run; empty for none |
| `-depth` | `18` | engine search depth in half-moves |
| `-threads`, `-hash` | engine default | engine threads and hash size in MB |
| `-cache` | `cache` | directory for saved evaluations; empty keeps them in memory |
| `-maia` | `auto` | model file; `auto` takes the largest in `./models`; empty for none |
| `-onnxruntime` | searched | path to the ONNX Runtime shared library |
| `-llm-url`, `-llm-model` | none | OpenAI-compatible endpoint and model for the written briefing |

The API key for the language model is read from the environment variable
`OPPOSITION_LLM_KEY`, so it does not appear on the command line.

## What to keep in mind when reading the results

- **Sample size.** A line seen in two games is an observation, not a habit.
  Raw game counts are shown everywhere for that reason, and the report warns
  below ten games.
- **Names must match exactly.** "Mueller, Hans" and "Müller,H" are two
  players. Spelling variants in a database are not merged.
- **Recency.** A game counts half after the chosen half-life (two years by
  default). Undated games are treated like the opponent's oldest dated game.
- **Engine numbers.** Evaluations are from a fixed-depth search. Differences
  of a few tenths of a pawn are noise, which is why "mistake" starts at half a
  pawn and "going wrong" in a trap at a full pawn.
- **Predictions describe a rating, not a person.** Maia-3 was trained on
  Lichess blitz games and works in Lichess blitz ratings, which tend to run
  higher than over-the-board Elo. The rating is taken from the database and
  not converted; adjust "Their rating" by hand. The model sees only the
  current position, not the moves before it, which makes it somewhat weaker
  than its published accuracy.
- **The written briefing can be wrong.** The language model is given the
  findings as text and told to add nothing, but nothing checks what it writes.
  The sections of the report are the source; the prose is a convenience.
- **Scans take time.** About half a second per position at depth 18 on an
  Apple M4 Pro. Evaluations are saved in `cache/`, so a second run is fast.
- **Promotions on the board always make a queen.**

## How it is built

One Go binary serves a JSON API and the embedded Svelte frontend.

| Package | Responsibility |
|---|---|
| `internal/core` | the logic, free of I/O: opening tree, recency weights, mistakes, traps, report sections |
| `internal/pgn` | reading PGN files into games |
| `internal/board` | legal moves and position keys for a single position |
| `internal/engine` | running a UCI engine as a subprocess; the evaluation cache |
| `internal/maia` | Maia-3 through ONNX Runtime |
| `internal/narrator` | the OpenAI-compatible client for the written briefing |
| `internal/api` | HTTP handlers and the background jobs |
| `web/` | the frontend |

A position is identified by the first four fields of its X-FEN (pieces, side
to move, castling rights, en passant only if a capture is possible).

## Development

```sh
make test     # Go tests and the frontend type check
make dev      # API on :8080; run "npm run dev" in web/ next to it
```

Building needs cgo because ONNX Runtime is C code. If linking fails on macOS
with "unknown architecture" errors, the default SDK is newer than the linker
understands; the Makefile sets `SDKROOT` for that, and a plain `go build` or
`go test` needs the same variable.

Tests that need Stockfish, ONNX Runtime or the model files skip when those
are missing.

The Maia tests compare this implementation with the official one. The
reference values in `testdata/maia/` come from `tools/maia_reference.py`,
which needs the official package (<https://github.com/CSSLab/maia3>)
installed in a Python environment.

## Licences of what this builds on

This repository has no licence of its own yet. Before publishing it, note:

- The board is [chessground](https://github.com/lichess-org/chessground),
  GPL-3.0.
- `internal/maia` re-implements the input encoding of
  [Maia-3](https://github.com/CSSLab/maia3), whose code is AGPL-3.0. The
  model files are a community conversion published as AGPL-3.0.
- Stockfish is GPL-3.0 and runs as a separate program.
