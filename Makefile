PGN ?= testdata/sample.pgn

# The Maia model runs through ONNX Runtime, which is C code, so building needs
# a working C linker. On Macs where the Command Line Tools default to an SDK
# newer than the installed linker understands, linking fails with "unknown
# architecture" errors; the unversioned MacOSX.sdk link is the matching one.
# Running "go build" or "go test" directly needs the same SDKROOT.
ifeq ($(shell uname),Darwin)
ifneq ($(wildcard /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk),)
export SDKROOT ?= /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk
endif
endif

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
