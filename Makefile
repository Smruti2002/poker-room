.DEFAULT_GOAL := help
SHELL := /bin/bash

SERVER := server
WEB := web
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

.PHONY: help
help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# --- Setup ---------------------------------------------------------------

.PHONY: install
install: ## Install all dependencies
	cd $(SERVER) && go mod download
	cd $(WEB) && npm ci

# --- Dev -----------------------------------------------------------------

.PHONY: dev-server
dev-server: ## Run the Go server on :8080
	cd $(SERVER) && LOG_LEVEL=debug go run ./cmd/server

.PHONY: dev-web
dev-web: ## Run the Vite dev server (proxies /api and /ws to :8080)
	cd $(WEB) && npm run dev

# --- Quality -------------------------------------------------------------

.PHONY: test
test: test-server test-web ## Run all tests

.PHONY: test-server
test-server: ## Go tests with the race detector
	cd $(SERVER) && go test -race -count=1 ./...

.PHONY: test-web
test-web: ## Web unit tests
	cd $(WEB) && npm test

.PHONY: e2e
e2e: ## Playwright end-to-end tests against the built server
	cd $(WEB) && npm run e2e

.PHONY: lint
lint: lint-server lint-web ## Run all linters

.PHONY: lint-server
lint-server: ## go vet + golangci-lint
	cd $(SERVER) && go vet ./...
	@command -v golangci-lint >/dev/null || { echo "golangci-lint not found: brew install golangci-lint"; exit 1; }
	cd $(SERVER) && golangci-lint run

.PHONY: lint-web
lint-web: ## Typecheck + oxlint
	cd $(WEB) && npm run typecheck && npm run lint

.PHONY: sim
sim: ## Play 100k bot hands and check engine invariants
	cd $(SERVER) && go run ./cmd/sim -hands 100000

.PHONY: protocol
protocol: ## Regenerate web/src/protocol/types.ts from the Go protocol types
	cd $(SERVER) && go run github.com/gzuidhof/tygo@v0.2.21 generate

.PHONY: fmt
fmt: ## Format Go code
	cd $(SERVER) && gofmt -w .

# --- Build ---------------------------------------------------------------

.PHONY: build
build: ## Build web app and server binary into ./bin
	cd $(WEB) && npm run build
	cd $(SERVER) && CGO_ENABLED=0 go build -ldflags "-s -w -X main.version=$(VERSION)" -o ../bin/server ./cmd/server

.PHONY: run
run: build ## Build, then serve the built app on :8080
	STATIC_DIR=$(WEB)/dist ./bin/server

.PHONY: ci
ci: lint test build ## Everything CI runs
