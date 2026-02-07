.PHONY: test coverage lint tidy build setup install-hooks fmt fmt-check pre-commit-check

# Run tests with coverage
test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

# Check coverage thresholds (per-file, per-package, total)
coverage: test
	go-test-coverage --config .testcoverage.yml

# Run linter
lint:
	golangci-lint run

# Check go.mod and go.sum are tidy
tidy:
	go mod tidy
	git diff --exit-code go.mod go.sum

# Format all Go code
fmt:
	@echo "Formatting Go code..."
	@gofmt -w .
	@echo "Code formatted successfully!"

# Check if code is formatted (use before committing)
fmt-check:
	@echo "Checking code formatting..."
	@if [ -n "$$(gofmt -l .)" ]; then \
		echo "❌ Unformatted files found:"; \
		gofmt -l .; \
		echo "Run 'make fmt' to fix formatting"; \
		exit 1; \
	fi
	@echo "✅ All code is properly formatted!"

# Run all pre-commit checks (format, lint, tidy, test)
pre-commit-check: fmt-check
	@echo "Running linter..."
	@golangci-lint run
	@echo "Checking go.mod/go.sum..."
	@go mod tidy && git diff --exit-code go.mod go.sum
	@echo "Running tests..."
	@go test -race ./...
	@echo ""
	@echo "✅ All pre-commit checks passed! Safe to commit."

# Build binary
build:
	go build -o bunny-api-proxy ./cmd/bunny-api-proxy

# Setup development environment (run once after cloning)
setup: install-hooks
	@echo "Development environment ready!"

# Install git hooks via lefthook
install-hooks:
	@echo "Installing lefthook..."
	@go install github.com/evilmartians/lefthook@latest
	@echo "Installing git hooks..."
	@$(shell go env GOPATH)/bin/lefthook install
	@echo "Git hooks installed successfully!"
