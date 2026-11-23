# Common tasks, the same as SAM's. `make help` lists them.

DGS_ENV ?= development
export DGS_ENV

.PHONY: help build run test test-short lint tidy-check vulncheck up down

help: ## list the tasks
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

build: ## build the dgs binary into bin/
	go build -o bin/dgs ./cmd/dgs

run: ## serve the API locally, in development unless DGS_ENV says otherwise
	go run ./cmd/dgs serve

test: ## run every test with the race detector
	go test -race ./...

test-short: ## run the tests that need no database or SAM
	go test -race -short ./...

lint: ## vet, format check and golangci-lint
	go vet ./...
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	golangci-lint run --allow-parallel-runners ./...

tidy-check: ## fail when go.mod or go.sum is not tidy
	go mod tidy -diff

vulncheck: ## report known vulnerabilities in the code's dependencies
	go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...

up: ## build and start the service with docker compose
	docker compose up -d --build --wait

down: ## stop the stack and discard its data
	docker compose down --volumes --remove-orphans
