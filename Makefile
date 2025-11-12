.PHONY: help build run dev test clean docker-up docker-down install-tools templ

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Available targets:'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  %-15s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install-tools: ## Install required development tools
	@echo "Installing templ..."
	@go install github.com/a-h/templ/cmd/templ@latest
	@echo "Tools installed successfully"

templ: ## Generate Go code from templ templates
	@echo "Generating templ templates..."
	@templ generate
	@echo "Templates generated successfully"

build: templ ## Build the application
	@echo "Building Kiln..."
	@go build -o kiln ./cmd/kiln
	@echo "Build complete"

run: templ ## Run the application locally
	@echo "Running Kiln..."
	@go run ./cmd/kiln

dev: ## Run with auto-reload (requires air or similar)
	@echo "Starting development server..."
	@air || go run ./cmd/kiln

test: ## Run tests
	@echo "Running tests..."
	@go test -v ./...

clean: ## Clean build artifacts
	@echo "Cleaning..."
	@rm -f kiln
	@rm -f cmd/kiln/kiln
	@echo "Clean complete"

docker-up: ## Start Docker containers
	@echo "Starting Docker containers..."
	@docker-compose up -d
	@echo "Containers started"

docker-down: ## Stop Docker containers
	@echo "Stopping Docker containers..."
	@docker-compose down
	@echo "Containers stopped"

docker-build: ## Build and start Docker containers
	@echo "Building and starting Docker containers..."
	@docker-compose up -d --build
	@echo "Containers built and started"

docker-logs: ## Show Docker logs
	@docker-compose logs -f

db-shell: ## Open PostgreSQL shell
	@docker-compose exec db psql -U postgres -d kiln

deps: ## Download Go dependencies
	@echo "Downloading dependencies..."
	@go mod download
	@echo "Dependencies downloaded"

tidy: ## Tidy Go modules
	@echo "Tidying modules..."
	@go mod tidy
	@echo "Modules tidied"

fmt: ## Format Go code
	@echo "Formatting code..."
	@go fmt ./...
	@echo "Code formatted"

.DEFAULT_GOAL := help
