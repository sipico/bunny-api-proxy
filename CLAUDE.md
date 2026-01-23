# CLAUDE.md

Project context and conventions for AI assistants working on this codebase.

## Project Overview

Bunny API Proxy is an API proxy for bunny.net that enables scoped/limited API keys. It sits between clients (e.g., ACME clients for DNS-01 validation) and the bunny.net API, validating requests against defined permissions before forwarding.

See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed technical decisions.

## Communication Conventions

### One topic at a time
Discussions should cover one question or decision at a time for clarity. Avoid presenting multiple choices simultaneously - work through decisions sequentially.

### Document trade-offs
Architecture and technology decisions require documented pros and cons. Present options in a clear format (tables work well) with rationale for recommendations.

### Scope management
Out-of-scope ideas and future enhancements go in [FUTURE_ENHANCEMENTS.md](FUTURE_ENHANCEMENTS.md), not into current work. When scope creep is detected, acknowledge the idea, add it to the future enhancements file, and refocus on the current task.

## Development Conventions

### Test-Driven Development
Write tests first, then implementation. Tests are the safety net since there's no local development environment.

### Code quality
- All code must pass `golangci-lint` (strict)
- All code must be formatted with `gofmt`
- Minimum 80% test coverage
- Run `govulncheck` for security

### Go idioms
- Follow standard Go project layout
- Use idiomatic Go patterns (Chi is chosen for being close to net/http)
- Code should be well-commented for maintainability
- Prefer simplicity over cleverness

### Git workflow
- All development happens via GitHub (no local environment)
- All validation happens via GitHub Actions
- Commit messages should be clear and descriptive
- Never commit secrets or API keys

## Build & Test Commands

```bash
# Format code
gofmt -w .

# Run linter
golangci-lint run

# Run tests with coverage
go test -race -cover ./...

# Security check
govulncheck ./...

# Build
go build -o bunny-proxy ./cmd/bunny-proxy

# Build Docker image
docker build -t bunny-api-proxy .
```

## Project Structure

```
cmd/bunny-proxy/     # Entry point
internal/            # Private application code
  proxy/             # Core proxy logic
  auth/              # Key validation, permissions
  storage/           # SQLite operations
  admin/             # Admin UI handlers
  bunny/             # bunny.net API client
  testutil/mockbunny/ # Mock server for testing
web/                 # Templates and static files
migrations/          # Database migrations
```

## Key Files

- [ARCHITECTURE.md](ARCHITECTURE.md) - Technical decisions and rationale
- [FUTURE_ENHANCEMENTS.md](FUTURE_ENHANCEMENTS.md) - Deferred features and ideas
- `.github/workflows/` - CI/CD pipeline (to be created)

## MVP Scope

DNS API only:
- List zones
- Get zone details
- List/Add/Delete records

Target use case: ACME DNS-01 validation with scoped permissions.
