.PHONY: help build test lint fmt vet security clean docker-build docker-run

# Default target
help:
	@echo "Available targets:"
	@echo "  build         - Build the binary"
	@echo "  test          - Run tests with coverage"
	@echo "  lint          - Run golangci-lint"
	@echo "  fmt           - Format code with gofmt"
	@echo "  vet           - Run go vet"
	@echo "  security      - Run govulncheck"
	@echo "  clean         - Remove build artifacts"
	@echo "  docker-build  - Build Docker image"
	@echo "  docker-run    - Run Docker container"

# Build the binary
build:
	@echo "Building bunny-proxy..."
	go build -o bunny-proxy ./cmd/bunny-proxy

# Run tests with coverage
test:
	@echo "Running tests with coverage..."
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...
	@go tool cover -func=coverage.txt | grep total

# Run golangci-lint
lint:
	@echo "Running golangci-lint..."
	golangci-lint run

# Format code
fmt:
	@echo "Formatting code..."
	gofmt -w .

# Run go vet
vet:
	@echo "Running go vet..."
	go vet ./...

# Run security checks
security:
	@echo "Running govulncheck..."
	@go install golang.org/x/vuln/cmd/govulncheck@latest
	govulncheck ./...

# Clean build artifacts
clean:
	@echo "Cleaning build artifacts..."
	rm -f bunny-proxy
	rm -f coverage.txt
	go clean

# Build Docker image
docker-build:
	@echo "Building Docker image..."
	docker build -t bunny-api-proxy:latest .

# Run Docker container
docker-run:
	@echo "Running Docker container..."
	docker run -p 8080:8080 bunny-api-proxy:latest
