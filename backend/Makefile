# Common tasks, the same as SAM's. `make help` lists them.

DGS_ENV ?= development
export DGS_ENV

# Tests run against DGS_TEST_DATABASE_URL when it is set (any PostGIS server
# tests may create databases on); otherwise against the compose test-db
# service, which `make test` starts.
TEST_DB ?= postgres://postgres:dgs@127.0.0.1:55433/postgres?sslmode=disable

# `make run` and `make migrate` use the compose db service unless
# DGS_DATABASE_URL names another database.
DGS_DATABASE_URL ?= postgres://postgres:dgs@127.0.0.1:5433/dgs?sslmode=disable
export DGS_DATABASE_URL

.PHONY: help build run migrate test test-short lint tidy-check vulncheck db-up up down

help: ## list the tasks
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

build: ## build the dgs binary into bin/
	go build -o bin/dgs ./cmd/dgs

run: ## serve the API locally, in development unless DGS_ENV says otherwise
	go run ./cmd/dgs serve

migrate: ## apply new migrations to DGS_DATABASE_URL
	go run ./cmd/dgs migrate

# SAM's module is private: go fetches it with git, as the signed-in user.
GOPRIVATE ?= github.com/dagrouper-school/*
export GOPRIVATE

test: ## run every test with the race detector, against the test database
ifdef DGS_TEST_DATABASE_URL
	go test -race ./...
else
	docker compose --profile test up -d --wait test-db
	DGS_TEST_DATABASE_URL='$(TEST_DB)' go test -race ./...
endif

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

db-up: ## start the development and test databases
	docker compose --profile test up -d --wait db test-db

up: ## build and start the service and its database with docker compose
	SAM_READ_TOKEN="$${SAM_READ_TOKEN:-$$(gh auth token)}" docker compose up -d --build --wait

down: ## stop the stack and discard its data
	docker compose down --volumes --remove-orphans
