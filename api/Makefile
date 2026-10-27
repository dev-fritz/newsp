GOLANGCI_LINT ?= $(shell command -v golangci-lint 2>/dev/null || echo $(shell go env GOPATH)/bin/golangci-lint)
COMPOSE := docker compose -f ../docker-compose.yml

.PHONY: tools infra-up infra-down migrate migrate-down migrate-version run worker reindex build test test-short lint fmt vet swag mocks check

tools: ## Install golangci-lint (swag and mockgen run through `go tool`)
	go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0

infra-up: ## Start Postgres (and friends) from the root compose file
	$(COMPOSE) up -d db

infra-down:
	$(COMPOSE) down

migrate:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down

migrate-version:
	go run ./cmd/migrate version

run:
	go run ./cmd/api

worker:
	go run ./cmd/worker

reindex: ## Enqueue a full embedding reindex; the worker does the work
	go run ./cmd/reindex

build:
	CGO_ENABLED=0 go build -trimpath -o bin/ ./cmd/...

test:
	go test -race ./...

test-short: ## Skips integration tests that need Docker
	go test -race -short ./...

lint:
	$(GOLANGCI_LINT) run ./...

fmt:
	gofmt -w .
	$(GOLANGCI_LINT) fmt ./...

vet:
	go vet ./...

swag:
	go tool swag init -g cmd/api/main.go -o docs --parseInternal --outputTypes go,json,yaml

mocks:
	go generate ./...

check: vet lint test build
	@test -z "$$(gofmt -l .)" || (echo "gofmt needed:" && gofmt -l . && exit 1)
