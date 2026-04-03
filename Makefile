# SmartHeart Makefile

.PHONY: help test test-backend test-backend-integration test-rag test-frontend test-admin test-coverage test-race build run clean docker-build docker-run lint fmt vet check-deps

# Default target
help:
	@echo "Available targets:"
	@echo "  check-deps        - Check system dependencies"
	@echo "  test              - Run the full local verification suite"
	@echo "  test-backend      - Run backend tests except sandbox-sensitive EKG integration"
	@echo "  test-backend-integration - Run backend EKG integration test"
	@echo "  test-rag          - Run RAG pipeline tests"
	@echo "  test-frontend     - Run frontend lint, tests, and production build"
	@echo "  test-admin        - Run admin typecheck, tests, and production build"
	@echo "  test-coverage     - Run backend tests with coverage report"
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
test: test-backend test-backend-integration test-rag test-frontend test-admin

test-backend:
	@echo "Running backend tests..."
	GOCACHE=/tmp/smartheart-gocache go test ./back-api/... -skip TestECGHandler_Integration_

test-backend-integration:
	@echo "Running backend EKG integration test..."
	GOCACHE=/tmp/smartheart-gocache go test ./back-api/workers -run TestECGHandler_Integration_

test-rag:
	@echo "Running RAG pipeline tests..."
	pytest rag_pipeline/tests -q

test-frontend:
	@echo "Running frontend lint..."
	cd frontend && npm run lint
	@echo "Running frontend tests..."
	cd frontend && npm run test
	@echo "Running frontend production build..."
	cd frontend && npm run build

test-admin:
	@echo "Running admin typecheck..."
	cd admin && npm run typecheck
	@echo "Running admin tests..."
	cd admin && npm run test
	@echo "Running admin production build..."
	cd admin && npm run build

test-coverage:
	@echo "Running backend tests with coverage..."
	GOCACHE=/tmp/smartheart-gocache go test -coverprofile=coverage.out ./back-api/... -skip TestECGHandler_Integration_
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-race:
	@echo "Running backend tests with race detection..."
	GOCACHE=/tmp/smartheart-gocache go test -race ./back-api/... -skip TestECGHandler_Integration_

# Build targets
build:
	@echo "Building application..."
	@CGO_ENABLED=0 go build -o bin/smartheart ./cmd

build-static:
	@echo "Building static binary..."
	@CGO_ENABLED=0 go build -a -installsuffix cgo -o bin/smartheart ./cmd

run:
	@echo "Running application..."
	@CGO_ENABLED=0 go run ./cmd

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
	@echo "Running Go linter..."
	golangci-lint run
	@echo "Running frontend lint..."
	cd frontend && npm run lint

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
	GOCACHE=/tmp/smartheart-gocache go test -bench=. -benchmem ./back-api/...

benchmark-ekg:
	@echo "Running EKG benchmarks..."
	GOCACHE=/tmp/smartheart-gocache go test -bench=. -benchmem ./back-api/workers/...

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
	GOCACHE=/tmp/smartheart-gocache go test ./back-api/... -skip TestECGHandler_Integration_
	GOCACHE=/tmp/smartheart-gocache go test ./back-api/workers -run TestECGHandler_Integration_
	pytest rag_pipeline/tests -q
	cd frontend && npm run lint
	cd frontend && npm run test
	cd frontend && npm run build
	cd admin && npm run typecheck
	cd admin && npm run test
	cd admin && npm run build

ci-build:
	@echo "Building for CI..."
	CGO_ENABLED=0 go build -o bin/smartheart ./cmd

# Environment setup
env-setup:
	@echo "Setting up environment..."
	cp .env.example .env
	@echo "Please edit .env file with your configuration"

# Quick development workflow
dev: fmt vet test-backend build
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
	@echo "  make test-backend                - Backend tests without EKG integration"
	@echo "  make test-backend-integration    - EKG integration test"
	@echo "  GOCACHE=/tmp/smartheart-gocache go test ./back-api/service -run TestRegister_"
