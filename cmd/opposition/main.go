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
	"opposition/internal/pgn"
	"opposition/internal/webui"
)

func main() {
	path := flag.String("pgn", "", "path to the PGN database (required)")
	addr := flag.String("addr", "localhost:8080", "address to listen on")
	flag.Parse()

	if *path == "" {
		fmt.Fprintln(os.Stderr, "usage: opposition -pgn <database.pgn> [-addr localhost:8080]")
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

	mux := http.NewServeMux()
	mux.Handle("/api/", api.NewHandler(games, report, time.Now))
	mux.Handle("/", webui.Handler())

	log.Printf("listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}
