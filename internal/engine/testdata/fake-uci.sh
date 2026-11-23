#!/bin/sh
# Minimal UCI engine for tests. Appends every command it receives to
# $FAKE_UCI_LOG. With FAKE_UCI_DIE=1 it exits instead of answering "go".
while IFS= read -r line; do
  if [ -n "$FAKE_UCI_LOG" ]; then printf '%s\n' "$line" >> "$FAKE_UCI_LOG"; fi
  case "$line" in
    uci) echo "id name fake"; echo "uciok" ;;
    isready) echo "readyok" ;;
    go*)
      if [ "$FAKE_UCI_DIE" = "1" ]; then exit 1; fi
      echo "info string searching"
      echo "info depth 1 seldepth 1 multipv 1 score cp 13 nodes 5 pv a2a3"
      echo "info depth 2 seldepth 2 multipv 1 score cp 99 lowerbound nodes 9 pv d2d4"
      echo "info depth 2 seldepth 3 multipv 1 score cp 42 nodes 10 pv e2e4 e7e5"
      echo "bestmove e2e4 ponder e7e5"
      ;;
    quit) exit 0 ;;
  esac
done
