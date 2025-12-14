.PHONY: help build run test clean docker-up docker-down docker-logs setup-admin migrate

# Default target
help:
	@echo "FindYourRoot Backend - Available Commands:"
	@echo "  make setup         - Initial setup (create .env, start DB, setup admin)"
	@echo "  make run           - Run the server locally"
	@echo "  make build         - Build the server binary"
	@echo "  make test          - Run tests"
	@echo "  make docker-up     - Start Docker services"
	@echo "  make docker-down   - Stop Docker services"
	@echo "  make docker-logs   - View Docker logs"
	@echo "  make setup-admin   - Setup/update admin user"
	@echo "  make migrate       - Run database migrations"
	@echo "  make clean         - Clean build artifacts"
	@echo "  make deploy        - Deploy to Cloud Run"

# Initial setup
setup:
	@if [ ! -f .env ]; then \
		echo "Creating .env from .env.example..."; \
		cp .env.example .env; \
		echo "⚠️  Please edit .env with your configuration!"; \
		exit 1; \
	fi
	@echo "Starting PostgreSQL..."
	@docker-compose up -d postgres
	@echo "Waiting for PostgreSQL to be ready..."
	@sleep 5
	@echo "Setting up admin user..."
	@go run cmd/setup-admin/main.go
	@echo "✅ Setup complete! Run 'make run' to start the server."

# Build the server
build:
	@echo "Building server..."
	@go build -o bin/server cmd/server/main.go
	@echo "✅ Build complete! Binary at bin/server"

# Run the server
run:
	@echo "Starting server..."
	@go run cmd/server/main.go

# Run tests
test:
	@echo "Running tests..."
	@go test -v ./...

# Docker commands
docker-up:
	@echo "Starting all services..."
	@if command -v docker-compose > /dev/null 2>&1; then \
		docker-compose up -d; \
	else \
		docker compose up -d; \
	fi

docker-down:
	@echo "Stopping all services..."
	@if command -v docker-compose > /dev/null 2>&1; then \
		docker-compose down; \
	else \
		docker compose down; \
	fi

docker-logs:
	@if command -v docker-compose > /dev/null 2>&1; then \
		docker-compose logs -f; \
	else \
		docker compose logs -f; \
	fi

docker-clean:
	@echo "Cleaning Docker resources..."
	@if command -v docker-compose > /dev/null 2>&1; then \
		docker-compose down -v; \
	else \
		docker compose down -v; \
	fi
	@docker system prune -f

# Setup admin user
setup-admin:
	@echo "Setting up admin user..."
	@go run cmd/setup-admin/main.go

# Run migrations
migrate:
	@echo "Running migrations..."
	@go run cmd/server/main.go -migrate-only

# Clean build artifacts
clean:
	@echo "Cleaning..."
	@rm -rf bin/
	@go clean

# Deploy to Cloud Run
deploy:
	@echo "Deploying to Cloud Run..."
	@./deploy.sh

# Development with hot reload (requires air)
dev:
	@if ! command -v air > /dev/null; then \
		echo "Installing air..."; \
		go install github.com/cosmtrek/air@latest; \
	fi
	@air

# Install development tools
tools:
	@echo "Installing development tools..."
	@go install github.com/cosmtrek/air@latest
	@echo "✅ Tools installed!"
