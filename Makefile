PGN ?= testdata/sample.pgn

.PHONY: test web build run dev

test:
	go test ./...
	cd web && npm run check

web:
	cd web && npm install && npm run build

build: web
	go build -o bin/opposition ./cmd/opposition

# Serve the built app on http://localhost:8080
run: build
	./bin/opposition -pgn $(PGN)

# API on :8080; start "cd web && npm run dev" next to it for the frontend with hot reload
dev:
	go run ./cmd/opposition -pgn $(PGN)
