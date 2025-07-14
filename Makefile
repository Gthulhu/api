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

# Build the application
build: deps
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
docker-build:
	@echo "Building Docker image..."
	docker build -t bss-metrics-api .

# Run Docker container
docker-run: docker-build
	@echo "Running Docker container..."
	docker run -p 8080:8080 bss-metrics-api

# Deploy to Kubernetes
k8s-deploy:
	@echo "Deploying to Kubernetes..."
	kubectl apply -f k8s/deployment.yaml

# Development commands
dev-setup: deps
	@echo "Setting up development environment..."
	go install github.com/gorilla/mux@latest

.DEFAULT_GOAL := help
