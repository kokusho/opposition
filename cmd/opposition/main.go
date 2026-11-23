// Command opposition serves the opening tree of the players in a PGN database.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"opposition/internal/api"
	"opposition/internal/core"
	"opposition/internal/engine"
	"opposition/internal/pgn"
	"opposition/internal/webui"
)

func main() {
	path := flag.String("pgn", "", "path to the PGN database (required)")
	addr := flag.String("addr", "localhost:8080", "address to listen on")
	enginePath := flag.String("engine", "stockfish", "UCI chess engine to run; empty to do without")
	depth := flag.Int("depth", 18, "engine search depth in half-moves")
	threads := flag.Int("threads", 0, "engine threads (0 keeps the engine's default)")
	hashMB := flag.Int("hash", 0, "engine hash table size in MB (0 keeps the engine's default)")
	flag.Parse()

	if *path == "" {
		fmt.Fprintln(os.Stderr, "usage: opposition -pgn <database.pgn> [flags]")
		flag.PrintDefaults()
		os.Exit(2)
	}

	file, err := os.Open(*path)
	if err != nil {
		log.Fatal(err)
	}
	started := time.Now()
	games, report, err := pgn.Load(file)
	file.Close()
	if err != nil {
		log.Fatalf("reading %s: %v", *path, err)
	}
	log.Printf("loaded %d games from %s in %s", report.Games, *path, time.Since(started).Round(time.Millisecond))
	for _, skipped := range report.Skipped {
		log.Printf("skipped game %d: %s", skipped.Number, skipped.Reason)
	}

	evaluator := startEngine(*enginePath, engine.Options{Depth: *depth, Threads: *threads, HashMB: *hashMB})

	mux := http.NewServeMux()
	mux.Handle("/api/", api.NewHandler(games, report, time.Now, evaluator))
	mux.Handle("/", webui.Handler())

	log.Printf("listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// startEngine runs the chess engine and wraps it in a cache. Without an
// engine the server still works; it just has no evaluations. It returns a nil
// interface in that case, which is what the API checks for.
func startEngine(path string, opts engine.Options) core.Evaluator {
	if path == "" {
		log.Print("no engine configured: evaluations and the mistake scan are off")
		return nil
	}
	started, err := engine.Start(path, opts)
	if err != nil {
		log.Printf("running without an engine: %v", err)
		return nil
	}
	log.Printf("engine %s ready, searching to depth %d", path, opts.Depth)
	return engine.NewCache(started)
}
