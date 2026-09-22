.PHONY: help config deps swagger run-api build fmt vet lint lint-fix check-golangci test cover vuln check migrate-create migrate-up migrate-down check-migrate docker-up docker-down docker-build

APP_NAME = paygate
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_TIME ?= $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS = -X 'paygate/internal/bootstrap.Version=$(VERSION)'

DATABASE_URL ?= postgresql://postgres:password@localhost:5432/paygate?sslmode=disable

help: ## Show this help
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "\033[36m%-20s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

config: ## Copy config.yml.example to config.yml if it doesn't exist
	@if [ ! -f config.yml ]; then cp config.yml.example config.yml; echo "Created config.yml from example"; else echo "config.yml already exists"; fi

deps: ## Download dependencies
	go mod download
	go mod tidy

swagger: ## Generate swagger documentation
	@which swag > /dev/null && swag init -g cmd/api/main.go -d . --parseDependency --parseInternal -o docs || echo "swag not installed, skipping swagger generation"

run-api: config ## Run the API server
	go run -ldflags "$(LDFLAGS)" cmd/api/main.go

build: ## Build the binary
	mkdir -p bin
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS) -w -s" -o bin/$(APP_NAME) cmd/api/main.go

fmt: ## Format source code
	go fmt ./...

vet: ## Run go vet
	go vet ./...

lint: check-golangci ## Run golangci-lint
	golangci-lint run

lint-fix: check-golangci ## Run golangci-lint and fix issues
	golangci-lint run --fix

check-golangci:
	@which golangci-lint > /dev/null || (echo "golangci-lint not installed, installing..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)

test: ## Run tests
	go test -v -race ./...

cover: ## Run tests with coverage
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

vuln: ## Run govulncheck
	@which govulncheck > /dev/null || go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

check: fmt vet test ## Run standard checks

migrate-create: check-migrate ## Create a new migration, usage: make migrate-create name=create_users
	@if [ -z "$(name)" ]; then echo "name parameter is required. Usage: make migrate-create name=create_users"; exit 1; fi
	migrate create -ext sql -dir db/migrations -seq $(name)

migrate-up: check-migrate ## Run all up migrations
	migrate -path db/migrations -database "$(DATABASE_URL)" up

migrate-down: check-migrate ## Run all down migrations (rolls back all)
	migrate -path db/migrations -database "$(DATABASE_URL)" down -all

check-migrate:
	@which migrate > /dev/null || (echo "migrate not installed, installing..." && go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest)

docker-up: ## Start docker dependencies
	docker-compose up -d

docker-down: ## Stop docker dependencies
	docker-compose down

docker-build: ## Build docker image
	docker build -t $(APP_NAME):$(VERSION) .
