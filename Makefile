.PHONY: test coverage lint tidy build

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

# Build binary
build:
	go build -o bunny-api-proxy ./cmd/bunny-api-proxy
