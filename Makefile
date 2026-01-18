.PHONY: help build run test lint docker up down logs clean

# Default target
help:
	@echo "Discord Prompter - Available targets:"
	@echo "  make build      - Build the bot binary"
	@echo "  make run        - Run the bot locally"
	@echo "  make test       - Run tests"
	@echo "  make lint       - Run linter (if golangci-lint is installed)"
	@echo "  make docker     - Build Docker image"
	@echo "  make up         - Start services with docker-compose"
	@echo "  make down       - Stop services"
	@echo "  make logs       - View bot logs"
	@echo "  make clean      - Clean build artifacts"

# Build binary
build:
	@echo "Building bot..."
	@mkdir -p bin
	@go build -o bin/bot ./cmd/bot
	@echo "Build complete: bin/bot"

# Run locally (requires Redis running)
run: build
	@./bin/bot --config config/config.yaml

# Run tests
test:
	@echo "Running tests..."
	@go test -v -race ./...

# Run linter
lint:
	@if command -v golangci-lint >/dev/null 2>&1; then \
		echo "Running linter..."; \
		golangci-lint run; \
	else \
		echo "golangci-lint not installed. Install from https://golangci-lint.run/"; \
	fi

# Build Docker image
docker:
	@echo "Building Docker image..."
	@docker build -t discord-prompter:latest .

# Start services
up:
	@echo "Starting services..."
	@docker-compose up -d
	@echo "Services started. Use 'make logs' to view logs."

# Stop services
down:
	@echo "Stopping services..."
	@docker-compose down

# View logs
logs:
	@docker-compose logs -f discord-prompter

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@go clean
	@echo "Clean complete"
