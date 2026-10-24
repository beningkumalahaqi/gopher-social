SHELL := /bin/bash
BIN := bin/api
GO ?= go
DATABASE_URL ?= postgresql://admin:password@localhost:5432/gopherdb

.PHONY: help run build test test-race lint fmt migrate-up migrate-down migrate-reset migrate-version seed smoke bench clean

help: ## List targets
	@grep -E '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

run: ## Start the API
	$(GO) run ./cmd/api

build: ## Compile the API to bin/api
	@mkdir -p bin
	$(GO) build -trimpath -o $(BIN) ./cmd/api

test: ## Unit + integration tests
	$(GO) test ./...

test-race: ## Tests under the race detector
	$(GO) test -race ./...

lint: ## Format check and vet
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:"; gofmt -l .; exit 1)
	$(GO) vet ./...

fmt: ## Format all Go files
	gofmt -w .

migrate-up: ## Apply all migrations
	$(GO) run ./cmd/migrate -dir=up

migrate-down: ## Roll back one migration
	$(GO) run ./cmd/migrate -dir=down

migrate-reset: ## Roll back every migration
	$(GO) run ./cmd/migrate -dir=drop

migrate-version: ## Print current schema version
	$(GO) run ./cmd/migrate -dir=version

seed: ## Destructive development seed
	$(GO) run ./cmd/seed

smoke: ## End-to-end smoke test against a running server
	python3 scripts/smoke.py

bench: ## Pagination benchmarks
	$(GO) test ./internal/pagination -run '^$$' -bench . -benchmem

clean: ## Remove build output
	rm -rf bin
