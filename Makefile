.PHONY: help build test test-int e2e lint migrate up down logs seed web-dev api-dev

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

build: ## Build the Go binaries
	go build -o bin/api ./cmd/api
	go build -o bin/poller ./cmd/poller

test: ## Go unit tests (no DB)
	go test ./...

test-int: ## Go integration tests (spins up Postgres via testcontainers)
	RUN_INTEGRATION=1 go test -tags=integration ./...

lint: ## Vet Go + lint web
	go vet ./...
	cd web && npm run lint

migrate: ## Apply DB migrations (goose)
	goose -dir migrations postgres "$(DATABASE_URL)" up

up: ## Build + start the full stack and wait until healthy
	docker compose up -d --build --wait

down: ## Stop the stack
	docker compose down

logs: ## Tail stack logs
	docker compose logs -f

seed: ## Seed the database (products, sources, OBI stores)
	go run ./cmd/api --seed

e2e: ## Run Playwright e2e against a running stack
	cd e2e && npm ci && npx playwright test

api-dev: ## Run the API locally
	go run ./cmd/api

web-dev: ## Run the frontend locally
	cd web && npm run dev
