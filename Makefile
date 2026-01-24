.PHONY: test coverage lint build

# Run tests with coverage
test:
	go test -race -coverprofile=coverage.txt -covermode=atomic ./...

# Check coverage thresholds (per-file, per-package, total)
coverage: test
	go-test-coverage --config .testcoverage.yml

# Run linter
lint:
	golangci-lint run

# Build binary
build:
	go build -o bunny-api-proxy ./cmd/bunny-api-proxy
