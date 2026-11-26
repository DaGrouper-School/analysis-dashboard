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

# `make run` calls the compose SAM, with the tenant file copied out of its
# volume. SAM verifies user tokens against the compose backend's key set,
# so calls made as a person need `make up`.
DGS_SAM_URL ?= http://127.0.0.1:8091
DGS_SAM_TENANT_FILE ?= .dev/tenant.json
DGS_SAM_ISSUER ?= http://backend:8090
export DGS_SAM_URL DGS_SAM_TENANT_FILE DGS_SAM_ISSUER

# Building SAM's image reads its private module.
COMPOSE_BUILD = SAM_READ_TOKEN="$${SAM_READ_TOKEN:-$$(gh auth token)}" docker compose

.PHONY: help build run migrate test test-short test-media lint tidy-check vulncheck db-up up down

help: ## list the tasks
	@grep -E '^[a-z-]+:.*## ' $(MAKEFILE_LIST) | awk -F':.*## ' '{printf "  %-12s %s\n", $$1, $$2}'

build: ## build the dgs binary into bin/
	go build -o bin/dgs ./cmd/dgs

run: .dev/tenant.json ## serve the API locally, in development unless DGS_ENV says otherwise
	go run ./cmd/dgs serve

.dev/tenant.json:
	$(COMPOSE_BUILD) up -d --build --wait db sam
	mkdir -p .dev
	docker compose cp sam:/config/tenant.json $@

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

test-media: ## test the S3 media store against the compose test-media service
	docker compose --profile test up -d --wait test-media
	DGS_TEST_MEDIA_URL=http://127.0.0.1:58333 go test -race -run TestS3 ./internal/media

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

db-up: ## start the development and test databases, and SAM
	$(COMPOSE_BUILD) --profile test up -d --build --wait db test-db sam

up: ## build and start the service, its database and SAM with docker compose
	$(COMPOSE_BUILD) up -d --build --wait

down: ## stop the stack and discard its data, SAM's included
	docker compose --profile test down --volumes --remove-orphans
	rm -f .dev/tenant.json
