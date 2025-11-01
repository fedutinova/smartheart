# SmartHeart Makefile

.PHONY: help test test-unit test-integration test-ekg test-coverage test-race build run clean docker-build docker-run lint fmt vet check-deps

# Default target
help:
	@echo "Available targets:"
	@echo "  check-deps        - Check system dependencies (g++, OpenCV, etc.)"
	@echo "  test              - Run all tests"
	@echo "  test-unit         - Run unit tests only"
	@echo "  test-integration  - Run integration tests only"
	@echo "  test-ekg          - Run EKG-specific tests"
	@echo "  test-coverage    - Run tests with coverage report"
	@echo "  test-race         - Run tests with race detection"
	@echo "  build             - Build the application"
	@echo "  run               - Run the application"
	@echo "  clean             - Clean build artifacts"
	@echo "  docker-build      - Build Docker image"
	@echo "  docker-run        - Run Docker container"
	@echo "  lint              - Run linter"
	@echo "  fmt               - Format code"
	@echo "  vet               - Run go vet"

# Dependency check
check-deps:
	@echo "Checking system dependencies..."
	@./scripts/install-dependencies.sh || true

# Test targets
test: test-unit test-integration

test-unit:
	@echo "Running unit tests..."
	go test -v -race -short ./internal/...

test-integration:
	@echo "Running integration tests..."
	go test -v -race -tags=integration ./internal/...

test-ekg:
	@echo "Running EKG tests..."
	go test -v -race ./internal/ekg/... ./internal/workers/...

test-coverage:
	@echo "Running tests with coverage..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race:
	@echo "Running tests with race detection..."
	go test -v -race ./...

# Build targets
build: check-deps
	@echo "Building application..."
	@CGO_ENABLED=1 go build -o bin/smartheart ./cmd

build-static:
	@echo "Building static binary..."
	@CGO_ENABLED=0 go build -a -installsuffix cgo -o bin/smartheart ./cmd

run: check-deps
	@echo "Running application..."
	@CGO_ENABLED=1 go run ./cmd

# Docker targets
docker-build:
	@echo "Building Docker image..."
	docker build -t smartheart:latest .

docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 smartheart:latest

docker-compose-up:
	@echo "Starting services with docker-compose..."
	docker-compose up --build

docker-compose-down:
	@echo "Stopping services..."
	docker-compose down

# Code quality targets
lint:
	@echo "Running linter..."
	golangci-lint run

fmt:
	@echo "Formatting code..."
	go fmt ./...

vet:
	@echo "Running go vet..."
	go vet ./...

# Development targets
dev-deps:
	@echo "Installing development dependencies..."
	go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest

setup-test-data:
	@echo "Setting up test data..."
	go run ./testdata/setup.go

# Clean targets
clean:
	@echo "Cleaning build artifacts..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean

clean-docker:
	@echo "Cleaning Docker artifacts..."
	docker system prune -f

# Database targets
db-migrate:
	@echo "Running database migrations..."
	# Add migration commands here

db-reset:
	@echo "Resetting database..."
	# Add database reset commands here

# Performance testing
benchmark:
	@echo "Running benchmarks..."
	go test -bench=. -benchmem ./...

benchmark-ekg:
	@echo "Running EKG benchmarks..."
	go test -bench=. -benchmem ./internal/ekg/... ./internal/workers/...

# Security testing
security:
	@echo "Running security checks..."
	gosec ./...

# Documentation
docs:
	@echo "Generating documentation..."
	godoc -http=:6060

# CI/CD targets
ci-test:
	@echo "Running CI tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out

ci-build:
	@echo "Building for CI..."
	CGO_ENABLED=1 go build -o bin/smartheart ./cmd

# Environment setup
env-setup:
	@echo "Setting up environment..."
	cp .env.example .env
	@echo "Please edit .env file with your configuration"

# Quick development workflow
dev: fmt vet test-unit build
	@echo "Development build complete"

# Production build
prod: clean fmt vet test test-coverage build-static
	@echo "Production build complete"

# Full test suite (for CI)
test-full: fmt vet lint test-coverage security
	@echo "Full test suite complete"

# Help for specific test packages
test-help:
	@echo "Test package examples:"
	@echo "  make test-ekg                    - EKG preprocessing tests"
	@echo "  go test ./internal/ekg/...       - EKG package only"
	@echo "  go test ./internal/workers/...   - Workers package only"
	@echo "  go test -run TestPreprocessImage - Specific test function"
