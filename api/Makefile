# Makefile for BSS Metrics API Server

.PHONY: help build run test test-strategies test-label-strategies clean docker-build docker-run client deps k8s-deploy

# Default target
help:
	@echo "Available commands:"
	@echo "  help                 - Show this help message"
	@echo "  deps                 - Install dependencies"
	@echo "  build                - Build the application"
	@echo "  run                  - Run the application"
	@echo "  client               - Run the example client"
	@echo "  test                 - Run tests"
	@echo "  test-strategies      - Test scheduling strategies API"
	@echo "  test-label-strategies- Test label-based scheduling strategies API"
	@echo "  clean                - Clean build artifacts"
	@echo "  docker-build         - Build Docker image"
	@echo "  docker-run           - Run Docker container"
	@echo "  k8s-deploy           - Deploy to Kubernetes"

# Install dependencies
deps:
	@echo "Installing dependencies..."
	go mod tidy
	go mod download

# Build frontend
build-web:
	@echo "Building frontend..."
	cd web && npm ci && npm run build

# Build the application
build: deps build-web
	@echo "Building application..."
	go build -o bin/api-server

# Run the application
run:
	@echo "Starting BSS Metrics API Server..."
	go run main.go

# Run the example client
client:
	@echo "Running example client..."
	go run client_example.go

# Run tests (when tests are added)
test:
	@echo "Running tests..."
	go test -v ./...

# Test strategies API
test-strategies:
	@echo "Testing scheduling strategies API..."
	./test/test_strategies.sh

# Test label-based strategies API
test-label-strategies:
	@echo "Testing label-based scheduling strategies API..."
	./test/test_label_strategies.sh

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f bin/api-server
	go clean

# Build Docker image
image:
	@echo "Building Docker image..."
	docker build -t 127.0.0.1:32000/gthulhu-api:latest .

# Run Docker container
docker-run: image
	@echo "Running Docker container..."
	docker run -p 8080:8080 127.0.0.1:32000/gthulhu-api

# Deploy to Kubernetes
k8s-deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s/deployment.yaml

# Development commands
dev-setup: deps
	@echo "Setting up development environment..."
	go install github.com/gorilla/mux@latest

.DEFAULT_GOAL := help

local-infra-up:
	@echo "Starting local infrastructure with Docker Compose..."
	docker-compose -f $(CURDIR)/deployment/local/docker-compose.infra.yaml up -d

local-infra-down:
	@echo "Stopping local infrastructure with Docker Compose..."
	docker-compose -f $(CURDIR)/deployment/local/docker-compose.infra.yaml down

local-run-manager:
	@echo "Running Manager locally..."
	go run main.go manager --config-dir $(CURDIR)/config/manager_config.toml --config-name manager_config

local-run-manger-migration:
	go install -tags 'mongodb' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
	migrate \
	-path $(CURDIR)/manager/migration \
	-database "mongodb://test:test@localhost:27017/manager?authSource=admin" \
	-verbose up


local-build-image.amd64:
	@echo "Building local Docker image for API Server..."
	docker build -f Dockerfile -t gthulhu-api:local .

gen-manager-swagger:
	@echo "Generating Swagger documentation for Manager..."
	swag init -g ./manager/cmd/cmd.go  -o docs/manager

local-kind-setup:
	sh $(CURDIR)/deployment/kind/local_setup.sh

local-kind-teardown:
	sh $(CURDIR)/deployment/kind/local_teardown.sh

gen-mock:
	@go install github.com/vektra/mockery/v3@v3.5.5
	@mockery

test-all:
	go test ./... -count=1 -p=1