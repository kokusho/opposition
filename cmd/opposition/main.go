// Command opposition serves the opening tree of the players in a PGN database.
package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"opposition/internal/api"
	"opposition/internal/core"
	"opposition/internal/engine"
	"opposition/internal/maia"
	"opposition/internal/narrator"
	"opposition/internal/pgn"
	"opposition/internal/webui"
)

// llmKeyEnv names the environment variable holding the API key for the
// language model, so the key never appears on the command line.
const llmKeyEnv = "OPPOSITION_LLM_KEY"

func main() {
	path := flag.String("pgn", "", "path to the PGN database (required)")
	addr := flag.String("addr", "localhost:8080", "address to listen on")
	enginePath := flag.String("engine", "stockfish", "UCI chess engine to run; empty to do without")
	depth := flag.Int("depth", 18, "engine search depth in half-moves")
	threads := flag.Int("threads", 0, "engine threads (0 keeps the engine's default)")
	hashMB := flag.Int("hash", 0, "engine hash table size in MB (0 keeps the engine's default)")
	maiaPath := flag.String("maia", "auto", "Maia-3 ONNX model for human-move predictions; \"auto\" takes the largest one in ./models, empty does without")
	runtimePath := flag.String("onnxruntime", "", "ONNX Runtime shared library (default: searched in the usual places)")
	cacheDir := flag.String("cache", "cache", "directory for saved engine evaluations; empty keeps them in memory only")
	llmURL := flag.String("llm-url", "", "base URL of an OpenAI-compatible API that writes the report up, e.g. http://localhost:11434/v1; empty to do without")
	llmModel := flag.String("llm-model", "", "model to request from -llm-url; an API key is read from "+llmKeyEnv)
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

	evaluator := startEngine(*enginePath, engine.Options{Depth: *depth, Threads: *threads, HashMB: *hashMB}, *cacheDir)

	mux := http.NewServeMux()
	mux.Handle("/api/", api.NewHandler(api.Config{
		Games:     games,
		Report:    report,
		Now:       time.Now,
		Engine:    evaluator,
		Predictor: loadMaia(*maiaPath, *runtimePath),
		Narrator:  llmNarrator(*llmURL, *llmModel),
	}))
	mux.Handle("/", webui.Handler())

	log.Printf("listening on http://%s", *addr)
	log.Fatal(http.ListenAndServe(*addr, mux))
}

// startEngine runs the chess engine and wraps it in a cache, kept in
// cacheDir across runs unless that is empty. Without an engine the server
// still works; it just has no evaluations. It returns a nil interface in that
// case, which is what the API checks for.
func startEngine(path string, opts engine.Options, cacheDir string) core.Evaluator {
	if path == "" {
		log.Print("no engine configured: evaluations, mistakes and traps are off")
		return nil
	}
	started, err := engine.Start(path, opts)
	if err != nil {
		log.Printf("running without an engine: %v", err)
		return nil
	}
	log.Printf("engine %s ready, searching to depth %d", path, opts.Depth)
	if cacheDir == "" {
		return engine.NewCache(started)
	}

	// One file per engine and depth: evaluations from another depth would
	// not be comparable.
	file := filepath.Join(cacheDir, fmt.Sprintf("%s-depth%d.jsonl", filepath.Base(path), opts.Depth))
	cache, err := engine.OpenCache(started, file)
	if err != nil {
		log.Printf("evaluations will not be saved: %v", err)
		return engine.NewCache(started)
	}
	log.Printf("%d saved evaluations loaded from %s", cache.Len(), file)
	return cache
}

// llmNarrator sets up the language model that writes reports up, or returns
// a nil interface when none is configured.
func llmNarrator(url, model string) core.Narrator {
	if url == "" {
		return nil
	}
	if model == "" {
		log.Print("-llm-url given without -llm-model: the written report is off")
		return nil
	}
	log.Printf("written reports come from %s at %s", model, url)
	return narrator.Client{BaseURL: url, Model: model, APIKey: os.Getenv(llmKeyEnv)}
}

// loadMaia opens the model that predicts human moves. Without it the server
// still works; it just has no predictions. It returns a nil interface in that
// case, which is what the API checks for.
func loadMaia(modelPath, runtimePath string) core.Predictor {
	if modelPath == "auto" {
		modelPath = largestMaiaModel()
	}
	if modelPath == "" {
		log.Print("no Maia model configured or found in ./models: move predictions are off")
		return nil
	}
	model, err := maia.Load(modelPath, runtimePath)
	if err != nil {
		log.Printf("running without move predictions: %v", err)
		return nil
	}
	log.Printf("Maia model %s loaded", modelPath)
	return model
}

// largestMaiaModel returns the most accurate Maia-3 model present in
// ./models, or "" if there is none. Bigger models predict a little better and
// still answer in milliseconds.
func largestMaiaModel() string {
	for _, size := range []string{"79m", "23m", "5m"} {
		path := "models/maia3-" + size + ".fp32.onnx"
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
