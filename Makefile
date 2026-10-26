# Makefile for Go REST API
#
# Node comparison:
# In Node.js, you define scripts inside package.json ("scripts": { "dev": "...", "build": "..." }).
# In Go, Makefiles are the standard convention for build automation and developer workflows.

.PHONY: all help run seed build test test-coverage lint migrate-up migrate-down migrate-status migrate-create sqlc docker-up docker-down tools-install

# Configuration variables
MIGRATIONS_DIR := migrations
DB_DSN ?= "postgres://postgres:postgres@localhost:5432/safar_trade?sslmode=disable"
BINARY_NAME := api

help: ## Show this help message
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-18s\033[0m %s\n", $$1, $$2}'

run: ## Run the API server locally
	go run ./cmd/api

seed: ## Seed the database with demo users, packages, subscriptions, and market data
	go run ./cmd/seed

build: ## Build the production binary into bin/
	go build -ldflags="-w -s" -o bin/$(BINARY_NAME) ./cmd/api

test: ## Run all unit and integration tests
	go test -v ./...

test-coverage: ## Run tests with code coverage report
	go test -v -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

lint: ## Run go vet and static analysis
	go vet ./...

sqlc: ## Generate type-safe Go code from SQL queries using sqlc
	sqlc generate -f db/sqlc.yaml

migrate-up: ## Run database migrations using goose
	goose -dir $(MIGRATIONS_DIR) postgres $(DB_DSN) up

migrate-down: ## Rollback the last database migration using goose
	goose -dir $(MIGRATIONS_DIR) postgres $(DB_DSN) down

migrate-status: ## Check the status of database migrations
	goose -dir $(MIGRATIONS_DIR) postgres $(DB_DSN) status

migrate-create: ## Create a new migration file (usage: make migrate-create name=add_tags)
	goose -dir $(MIGRATIONS_DIR) create $(name) sql

docker-up: ## Start PostgreSQL container in background
	docker compose up -d

docker-down: ## Stop PostgreSQL container
	docker compose down

tools-install: ## Install developer CLI tools (goose and sqlc)
	go install github.com/pressly/goose/v3/cmd/goose@latest
	go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
