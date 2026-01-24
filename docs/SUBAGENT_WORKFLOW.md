# Sub-Agent Workflow Guide

This document describes how to use Haiku sub-agents for cost-effective code implementation while maintaining quality.

## Overview

**Cost savings:** Using Haiku for implementation tasks saves ~83% compared to Opus.

**Division of labor:**
- **Opus (coordinator):** Design, specs, issue creation, code review, merging
- **Haiku (sub-agent):** Implementation, testing, documentation updates

## Workflow Phases

### Phase 1: Task Breakdown (Opus)

Break large tasks into small, focused sub-tasks:

| Good Sub-Task | Bad Sub-Task |
|---------------|--------------|
| "Add GetZone method + tests" | "Implement entire API client" |
| "Create types.go with Zone, Record structs" | "Create all files for the package" |
| "Fix coverage for handleDeleteRecord" | "Improve code quality" |

**Rules:**
- Each sub-task should modify 1-3 files max
- Each sub-task should be completable in one session
- Dependencies between tasks must be explicit

### Phase 2: Issue Creation (Opus)

Create a GitHub issue for each sub-task with this structure:

```markdown
## Overview
One sentence describing the task.

## Scope
**ONLY implement what is specified here. Nothing else.**

### Files to Create
- `path/to/file.go`

### Files to Modify
- `path/to/existing.go`

### Reference Files (read-only)
- `path/to/reference.go` - description of what to learn from it

---

## Specification

[Detailed spec with code signatures, behavior, error handling]

---

## Acceptance Criteria
- [ ] Code compiles: `go build ./...`
- [ ] Tests pass: `go test ./...`
- [ ] Coverage meets threshold: `make coverage`
- [ ] Linter passes: `golangci-lint run`
- [ ] CI passes (check after push)

---

## Communication Requirements

Post comments on this issue for:
1. **Implementation Plan** - before starting
2. **Design Decisions** - any trade-offs or choices made
3. **Blockers/Questions** - if stuck
4. **Completion Summary** - what was done
5. **Token Usage** - format: `Input: X, Output: Y, Total: Z`

---

## Constraints
- Do NOT modify [list files that should not be touched]
- Do NOT implement [list things explicitly out of scope]
```

### Phase 3: Sub-Agent Prompt (Opus)

The prompt should be **minimal** - the issue is the source of truth.

**Template:**
```
Implement GitHub issue #XX for [repo].

Read the issue: gh issue view XX --repo [owner/repo]

BRANCH: `claude/issue-XX-[SESSION_ID]` (use this exact name)

WORKFLOW:
1. Post comment: implementation plan
2. Read reference files from issue
3. Implement ONLY what the issue specifies
4. Post comment: design decisions (if any)
5. Validate locally: make coverage && golangci-lint run
6. Create branch, commit, push
7. Create PR to main
8. Wait for CI to pass (check with: gh pr checks XX --repo [owner/repo])
9. Post comment: completion summary
10. Post comment: token usage (Input: X, Output: Y, Total: Z)

If CI fails, fix the issue and push again before declaring complete.
```

**Critical elements:**
- Explicit branch name (never use `<your-session-id>`)
- Require CI validation
- Standardized token reporting format

### Phase 4: Execution Patterns

#### Sequential Execution (when tasks modify same files)

```
Task 1 (types.go)
    → merge PR
    → Task 2 (client.go imports types)
        → merge PR
        → Tasks 3,4,5 can run in parallel if they modify different methods
```

#### Parallel Execution (when tasks modify different files)

```
Task A (handlers.go)  ─┐
Task B (server.go)    ─┼─→ merge all PRs
Task C (types.go)     ─┘
```

#### Conflict Resolution (when parallel tasks touch same files)

After merging first PR, remaining PRs will have conflicts:

```
Spawn sub-agent with prompt:
"Resolve merge conflicts for PR #XX.
Branch: claude/issue-XX-[SESSION_ID]
Run: git fetch origin main && git merge origin/main
Keep ALL code from both branches.
Validate and push."
```

### Phase 5: Review and Merge (Opus)

1. **Check CI status:** `gh pr checks XX --repo [owner/repo]`
2. **Review diff:** `gh pr diff XX --repo [owner/repo]`
3. **Merge:** `gh pr merge XX --repo [owner/repo] --merge`
4. **Update main:** `git checkout main && git pull`

## Lessons Learned

### What Works Well

| Practice | Why It Works |
|----------|--------------|
| Detailed issue specs | Sub-agent has clear boundaries |
| Explicit branch names | Avoids session ID confusion |
| Issue comments for communication | Creates audit trail |
| `make coverage` locally | Catches CI failures early |
| Small focused tasks | Easier to review, less conflict |

### What to Avoid

| Anti-Pattern | Problem | Solution |
|--------------|---------|----------|
| `<your-session-id>` in prompts | Sub-agent guesses wrong ID | Use explicit branch name |
| Skipping CI check | Merge failures | Require CI pass before done |
| Large multi-file tasks | Merge conflicts, hard to review | Break into smaller tasks |
| Vague scope | Sub-agent adds unrequested features | Explicit "do NOT" list |
| No token reporting | Can't track costs | Standardized format required |

### Parallel Execution Guidelines

| Scenario | Recommendation |
|----------|----------------|
| Tasks modify different packages | Safe to parallelize |
| Tasks modify different files in same package | Usually safe |
| Tasks add methods to same file | Sequential or expect conflicts |
| Tasks modify same function | Must be sequential |

## Token Usage Tracking

Sub-agents should report in this format:
```
## Token Usage
- Input: ~X,XXX tokens
- Output: ~X,XXX tokens
- Total: ~X,XXX tokens
```

Typical task sizes:
| Task Type | Typical Tokens |
|-----------|---------------|
| Types/structs only | 8,000 - 12,000 |
| Single method + tests | 15,000 - 25,000 |
| Bug fix with investigation | 20,000 - 40,000 |
| Merge conflict resolution | 10,000 - 20,000 |

## Checklist for Coordinator

Before spawning sub-agent:
- [ ] Issue created with full spec
- [ ] Dependencies merged to main
- [ ] Explicit branch name ready
- [ ] Clear acceptance criteria

After sub-agent completes:
- [ ] CI passes
- [ ] Code reviewed
- [ ] PR merged
- [ ] Token usage recorded

## Example: Complete Sub-Agent Session

**Issue #42:** Add DeleteZone method

**Prompt sent to Haiku:**
```
Implement GitHub issue #42 for sipico/bunny-api-proxy.

Read the issue: gh issue view 42 --repo sipico/bunny-api-proxy

BRANCH: `claude/issue-42-JNZLv`

WORKFLOW:
1. Post comment: implementation plan
2. Implement DeleteZone in client.go
3. Add TestDeleteZone to client_test.go
4. Validate: make coverage && golangci-lint run
5. Push and create PR
6. Verify CI passes: gh pr checks [PR#] --repo sipico/bunny-api-proxy
7. Post comments: design decisions, completion summary, token usage
```

**Sub-agent posts on issue:**
1. Implementation plan
2. Design decision: "Used ErrNotFound for 404, consistent with other methods"
3. Completion: "Added DeleteZone + 4 test cases, coverage 87%"
4. Tokens: "Input: 18,500, Output: 4,200, Total: 22,700"

**Coordinator:** Reviews PR, CI passes, merges.
