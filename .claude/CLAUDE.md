# CLAUDE.md

Project context and conventions for AI assistants working on this codebase.

## Project Overview

Bunny API Proxy is an API proxy for bunny.net that enables scoped/limited API keys. It sits between clients (e.g., ACME clients for DNS-01 validation) and the bunny.net API, validating requests against defined permissions before forwarding.

See [ARCHITECTURE.md](../ARCHITECTURE.md) for detailed technical decisions.

## Communication Conventions

### One topic at a time
Discussions should cover one question or decision at a time for clarity. Avoid presenting multiple choices simultaneously - work through decisions sequentially.

### Document trade-offs
Architecture and technology decisions require documented pros and cons. Present options in a clear format (tables work well) with rationale for recommendations.

### Scope management
Out-of-scope ideas and future enhancements go in [FUTURE_ENHANCEMENTS.md](dev/FUTURE_ENHANCEMENTS.md), not into current work. When scope creep is detected, acknowledge the idea, add it to the future enhancements file, and refocus on the current task.

## Development Conventions

### Test-Driven Development
Write tests first, then implementation. Tests are the safety net.

### Code quality
- All code must pass `golangci-lint` (strict)
- All code must be formatted with `gofmt`
- Minimum 85% test coverage (aim for 95%)
- Run `govulncheck` for security

### Go idioms
- Follow standard Go project layout
- Use idiomatic Go patterns (Chi is chosen for being close to net/http)
- Code should be well-commented for maintainability
- Prefer simplicity over cleverness

### Git workflow
- **ALWAYS** run `make fmt` before committing (auto-formats all Go code)
- Run `make pre-commit-check` before pushing to catch all issues
- Commit messages should be clear and descriptive
- Never commit secrets or API keys
- Never use `--no-verify` to skip git hooks
- CI validates all changes via GitHub Actions
- Pre-commit hooks auto-format code and enforce linting
- Pre-push hooks run final tests and validation

### Development Setup

Run once after cloning to install Git pre-commit hooks:

```bash
make setup
```

This installs [lefthook](https://github.com/evilmartians/lefthook) hooks that automatically:
- Format Go code with `gofmt` before each commit
- Run `golangci-lint` on staged files
- Validate `go.mod` and `go.sum` are tidy
- Run full tests and linting before push

### Code Formatting (CRITICAL FOR AI AGENTS)

**ALWAYS format code before committing.** Git hooks will auto-format, but agents should be explicit:

```bash
# Format all Go code (run BEFORE staging/committing)
make fmt

# Check if code is formatted without changing files
make fmt-check

# Run ALL pre-commit checks (format + lint + tidy + test)
make pre-commit-check
```

**Workflow for AI agents:**
1. Write/modify code
2. Run `make fmt` to format
3. Run `make pre-commit-check` to validate
4. Stage files with `git add`
5. Commit (hooks will run automatically)
6. Push (pre-push hooks will validate again)

**NEVER skip formatting.** Unformatted code will be rejected by CI.

### Local Validation

Run before pushing to catch issues early:

```bash
# Full validation (recommended before push)
make pre-commit-check

# Individual checks
make fmt-check    # Check formatting only
make lint         # Run golangci-lint
make tidy         # Check go.mod/go.sum
make test         # Run tests with coverage
```

Pre-commit hooks will auto-format and validate, but pre-push hooks provide final safety checks.

### GitHub CLI

Use `gh` with `--repo` for GitHub operations:

```bash
gh issue list --repo sipico/bunny-api-proxy
gh pr view 123 --repo sipico/bunny-api-proxy
gh run list --repo sipico/bunny-api-proxy
gh run view <run-id> --repo sipico/bunny-api-proxy --log-failed
```

**Creating Pull Requests:**

Use `--base` and `--head` flags to explicitly specify the branches:

```bash
gh pr create --repo sipico/bunny-api-proxy \
  --base main \
  --head <your-branch-name> \
  --title "Your PR Title" \
  --body "$(cat <<'EOF'
## Summary
Your PR description here

## Changes
- Change 1
- Change 2
EOF
)"
```

This is especially useful when:
- Working with local git remotes that don't point to GitHub.com
- The `--head` flag ensures your feature branch is used instead of relying on origin detection
- Always use `--base main` for consistency, and replace `<your-branch-name>` with your actual branch name

## Project Structure

```
cmd/bunny-api-proxy/    # Entry point
internal/               # Private application code
  proxy/                # Core proxy logic
  auth/                 # Key validation, permissions
  storage/              # SQLite operations
  admin/                # Admin API handlers
  bunny/                # bunny.net API client
  testutil/mockbunny/   # Mock server for testing
```

## Key Files

- [ARCHITECTURE.md](../ARCHITECTURE.md) - Technical decisions and rationale
- [FUTURE_ENHANCEMENTS.md](dev/FUTURE_ENHANCEMENTS.md) - Deferred features and ideas
- [SUBAGENT_WORKFLOW.md](dev/SUBAGENT_WORKFLOW.md) - Cost-effective sub-agent patterns
- `.github/workflows/ci.yml` - CI/CD pipeline
- `.lefthook.yml` - Git pre-commit hook configuration

## Sub-Agent Workflow

For implementation tasks, use faster/cheaper sub-agent models to save on costs:

1. **Coordinator (larger model):** Creates detailed GitHub issues with specs
2. **Sub-agent (smaller model):** Implements code, tests, runs validation
3. **Coordinator (larger model):** Reviews and merges PRs

See [dev/SUBAGENT_WORKFLOW.md](dev/SUBAGENT_WORKFLOW.md) for detailed patterns.

**Quick reference:**
- Issue = source of truth (full spec)
- Prompt = minimal (just workflow steps)
- Always use explicit branch names (never `<your-session-id>`)
- Sub-agent must verify CI passes before declaring complete
- Token usage reported as: `Input: X, Output: Y, Total: Z`

## MVP Scope

DNS API only:
- List zones
- Get zone details
- List/Add/Delete records

Target use case: ACME DNS-01 validation with scoped permissions.
