# Sub-Agent Workflow Guide

This document describes how to use Haiku sub-agents for cost-effective code implementation while maintaining quality.

## Overview

**Cost savings:** Using Haiku for implementation tasks saves **~99% compared to Opus** (measured: $17.84 vs $2,495.72 for 19 subagent tasks).

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

**CRITICAL: Coordinator must provide explicit values, not placeholders.**

**Template:**
```
Implement GitHub issue #XX for sipico/bunny-api-proxy.

Read the issue: gh issue view XX --repo sipico/bunny-api-proxy

## Git Configuration (use EXACTLY these values)

Worktree: /home/user/bunny-api-proxy-wt-XX
Branch: claude/issue-XX-YYYYY
Session ID: YYYYY

## Setup Commands (copy-paste exactly)

```bash
cd /home/user/bunny-api-proxy
git fetch origin main
git worktree add /home/user/bunny-api-proxy-wt-XX -b claude/issue-XX-YYYYY origin/main
cd /home/user/bunny-api-proxy-wt-XX
```

## Workflow

1. Post comment on issue: implementation plan
2. Read reference files listed in issue
3. Implement ONLY what the issue specifies
4. Post comment: design decisions (if any)
5. Validate: go test -race -cover ./... && gofmt -w . && golangci-lint run
6. Commit and push:
   git add -A
   git commit -m "Description of changes"
   git push -u origin claude/issue-XX-YYYYY
7. Wait for CI to pass (sleep 60, then check)
8. Create PR ONLY after CI passes
9. Verify PR checks pass: gh pr checks [PR#] --repo sipico/bunny-api-proxy --watch
10. Post comment: completion summary
11. Post comment: token usage (Input: X, Output: Y, Total: Z)
12. Cleanup: cd /home/user/bunny-api-proxy && git worktree remove /home/user/bunny-api-proxy-wt-XX

If CI fails, fix and push again. Do NOT create PR until CI passes.
If PR checks are pending, wait. Do NOT declare success until PR is green.
```

**Critical elements:**
- **Explicit worktree path** (never use `${ISSUE_NUM}` - use actual number)
- **Explicit branch name** (never use `[SESSION_ID]` - use actual ID)
- **Explicit session ID** (coordinator provides this)
- **Always use `--repo sipico/bunny-api-proxy`** with ALL gh commands (git remote is proxied, gh cannot auto-detect)
- Require CI validation before PR creation
- **Require PR check verification** - don't declare success until PR shows green
- Standardized token reporting format

### Common gh CLI Commands (always include --repo)

```bash
# View issue
gh issue view 72 --repo sipico/bunny-api-proxy

# Create PR
gh pr create --repo sipico/bunny-api-proxy --base main --head claude/issue-72-XXXXX --title "..." --body "..."

# Check PR status
gh pr checks 123 --repo sipico/bunny-api-proxy --watch

# Merge PR
gh pr merge 123 --repo sipico/bunny-api-proxy --merge

# List CI runs
gh run list --repo sipico/bunny-api-proxy --branch claude/issue-72-XXXXX --limit 1

# Comment on issue
gh issue comment 72 --repo sipico/bunny-api-proxy --body "..."
```

**Why `--repo` is required:** The git remote uses a local proxy (`http://127.0.0.1:XXXXX/git/...`), so `gh` cannot auto-detect the GitHub repository. Without `--repo`, commands will fail.

### Phase 3a: Coordinator Session Management

**The coordinator MUST track and distribute their session ID.**

When starting coordination for a parent issue, establish:

```markdown
## Coordination Session

**Coordinator Session ID:** `NBGre`

All sub-issues will use this session ID in branch names.

| Issue | Title | Branch Name | Worktree Path |
|-------|-------|-------------|---------------|
| #72 | Proxy core types | `claude/issue-72-NBGre` | `/home/user/bunny-api-proxy-wt-72` |
| #73 | HTTP handlers | `claude/issue-73-NBGre` | `/home/user/bunny-api-proxy-wt-73` |
| #74 | Integration | `claude/issue-74-NBGre` | `/home/user/bunny-api-proxy-wt-74` |
```

**Why this matters:**
- Git push authorization requires branch names ending with coordinator's session ID
- Agents cannot determine the session ID themselves
- Explicit values prevent 403 errors and branch recreation

### Phase 3b: Handling Dependencies Between Issues

**For issues that depend on other issues being merged first:**

Add this section to dependent issues:

```markdown
## ⚠️ DEPENDENCIES - Pre-Flight Check Required

**This issue depends on:** #72, #73

### Before Starting (REQUIRED)

```bash
cd /home/user/bunny-api-proxy
git fetch origin main
git log --oneline origin/main | head -10
```

**Verify these commits exist on main:**
- [ ] "Proxy: Implement core types" (from #72)
- [ ] "Proxy: Implement HTTP handlers" (from #73)

### If Dependencies NOT Merged

**DO NOT:**
- ❌ Create stub implementations
- ❌ Proceed with implementation
- ❌ Create worktree

**DO:**
- ✅ Comment on this issue: "Blocked: waiting for #72, #73 to merge"
- ✅ Wait for coordinator to confirm merges
- ✅ Re-run pre-flight check after notification

### Why This Matters

Creating stub implementations will:
- Fail coverage thresholds (stubs = ~15% coverage)
- Create duplicate type definitions
- Require manual conflict resolution
- Waste CI runs
```

**Coordinator responsibility:** Do not spawn agents for dependent issues until dependencies are merged.

### Phase 3c: Coordination Issue Structure

Parent coordination issues (that spawn multiple sub-issues) should include these sections:

| Section | Purpose |
|---------|---------|
| **Session ID Block** | Coordinator's session ID, branch naming table with explicit values |
| **Dependency Graph** | Which sub-issues depend on others (blocks/blocked-by) |
| **Execution Strategy** | Parallel vs sequential, with rationale |
| **Expected Merge Conflicts** | Files likely to conflict (e.g., router.go), resolution strategy |
| **Shared Test Infrastructure** | Mock strategy (first issue creates, others import) |
| **Progress Tracking** | Checklist of sub-issues with status |
| **Quality Gates** | Coverage target (80% aim, 75% minimum), error path testing |

**Why this matters:**
- Proactive conflict identification saves CI runs
- Shared mock strategy prevents interface drift
- Explicit dependencies prevent stub implementations
- Progress tracking enables coordinator intervention

**Examples:** See retrospectives for detailed coordination examples:
- `docs/retrospectives/proxy-package-coordination-20260125.md`
- `docs/retrospectives/admin-package-coordination-20260125.md`

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

### Phase 4a: Git Worktree Isolation for Parallel Execution

**Problem:** When running multiple sub-agents in parallel, they all share the same working directory (`/home/user/project`). This causes:
- Agents see each other's uncommitted changes
- File interference and scope violations
- Race conditions on go.mod/go.sum
- Confused git state

**Solution:** Use git worktree to give each agent its own isolated directory.

#### How Git Worktree Works

Git worktree creates separate working directories that share the same `.git` database:

```
/home/user/bunny-api-proxy/          # Main checkout (main branch)
/home/user/bunny-api-proxy-wt-48/    # Worktree for issue #48
/home/user/bunny-api-proxy-wt-49/    # Worktree for issue #49
/home/user/bunny-api-proxy-wt-50/    # Worktree for issue #50
```

Each worktree:
- Has its own files (complete isolation)
- Can be on a different branch
- Shares git objects (efficient, no duplication)
- Can run builds/tests independently

#### Issue Template with Worktree Setup

Add this section to issues when parallel execution is planned.

**CRITICAL: Use explicit values, not placeholders. The coordinator fills in actual numbers.**

```markdown
## ⚠️ CRITICAL: Git Worktree Workflow

**You MUST use git worktree to avoid interfering with other parallel sub-agents.**

### Git Configuration (use EXACTLY these values)

| Setting | Value |
|---------|-------|
| Issue Number | 72 |
| Session ID | NBGre |
| Worktree Path | `/home/user/bunny-api-proxy-wt-72` |
| Branch Name | `claude/issue-72-NBGre` |

### Setup Commands (copy-paste exactly)

```bash
cd /home/user/bunny-api-proxy
git fetch origin main
git worktree add /home/user/bunny-api-proxy-wt-72 -b claude/issue-72-NBGre origin/main
cd /home/user/bunny-api-proxy-wt-72
```

### Implementation

Work ONLY in the worktree directory. All commands run here.

### Push and Verify CI (BEFORE creating PR)

```bash
git add -A
git commit -m "Proxy: Implement core types and structs"
git push -u origin claude/issue-72-NBGre

# Wait for CI
sleep 60

# Check CI status
gh run list --repo sipico/bunny-api-proxy --branch claude/issue-72-NBGre --limit 1

# If CI fails: fix, commit, push, check again
# Do NOT create PR until CI passes
```

### Create PR (ONLY after CI passes)

```bash
gh pr create --repo sipico/bunny-api-proxy \
  --base main \
  --head claude/issue-72-NBGre \
  --title "Proxy: Implement core types and structs" \
  --body "Closes #72"
```

### Verify PR Checks (BEFORE declaring success)

```bash
gh pr checks <PR_NUMBER> --repo sipico/bunny-api-proxy --watch
# Wait until ALL checks show "pass"
# Do NOT report completion until PR is green
```

### Cleanup (AFTER PR is green)

```bash
cd /home/user/bunny-api-proxy
git worktree remove /home/user/bunny-api-proxy-wt-72
```

### Why This Matters

- **Worktree isolation**: Prevents interference between parallel sub-agents
- **Explicit values**: Prevents session ID errors and 403 push failures
- **CI before PR**: Ensures quality, avoids fixup commits
- **PR verification**: Confirms PR is mergeable before reporting success
```

#### Worktree Management Commands

**List all worktrees:**
```bash
git worktree list
```

**Remove a worktree:**
```bash
git worktree remove /path/to/worktree
# Or force remove if there are uncommitted changes:
git worktree remove --force /path/to/worktree
```

**Cleanup orphaned worktrees:**
```bash
git worktree prune
```

#### When to Use Worktrees

| Scenario | Use Worktree? | Reason |
|----------|---------------|---------|
| Single sequential task | No | Unnecessary overhead |
| 2+ parallel tasks | **Yes** | Prevents interference |
| Tasks on different packages | Yes | Even if unlikely to conflict |
| Tasks with go.mod changes | **Yes (critical)** | Avoids race conditions |
| Quick fix on existing branch | No | Just switch branches |

#### Go Module Conflicts with Worktrees

When parallel tasks both add dependencies, they'll conflict on `go.mod` and `go.sum` during merge.

**Resolution strategy:**

```bash
# After first PR merges, rebase remaining PRs:
cd /path/to/worktree
git fetch origin main
git rebase origin/main

# Conflict on go.mod/go.sum?
git checkout --theirs go.mod go.sum
go mod tidy
git add go.mod go.sum
git rebase --continue

# Verify builds
go build ./...
go test ./...

# Push update
git push -f origin [branch-name]

# Wait for CI, then merge PR
```

**Key insight:** Let `go mod tidy` regenerate the files rather than manually merging. Go's tooling knows the correct dependency graph.

### Phase 5: Review and Merge (Opus)

1. **Check CI status:** `gh pr checks XX --repo [owner/repo]`
2. **Review diff:** `gh pr diff XX --repo [owner/repo]`
3. **Merge:** `gh pr merge XX --repo [owner/repo] --merge`
4. **Update main:** `git checkout main && git pull`

## Spec Detail Levels

Based on experience from two sub-agent sessions (mockbunny and bunny client), there are two approaches to spec detail:

### Prescriptive Specs (More Detail)

Used in mockbunny session. Issue includes near-complete code:

```markdown
## Implementation

```go
func (s *Server) handleListZones(w http.ResponseWriter, r *http.Request) {
    // Parse query parameters
    page := 1
    perPage := 1000
    // ... full implementation provided
}
```

## Test Cases

```go
func TestListZones_Empty(t *testing.T) {
    // ... full test code provided
}
```
```

**Pros:**
- Highly predictable output
- Less agent interpretation
- Faster implementation (copy-paste-adjust)

**Cons:**
- More coordinator effort upfront
- Less agent learning/adaptation
- May miss better solutions

**Best for:** Standard patterns, boilerplate, well-understood problems

### Abstract Specs (Less Detail)

Used in bunny client session. Issue includes signatures and behavior:

```markdown
## Specification

```go
// GetZone retrieves a single DNS zone by ID, including all its records.
func (c *Client) GetZone(ctx context.Context, id int64) (*Zone, error)
```

### Behavior
- Returns ErrNotFound for 404
- Returns ErrUnauthorized for 401
- Includes all records in zone
```

**Pros:**
- Less coordinator effort
- Agent can find optimal implementation
- Better for complex logic

**Cons:**
- Less predictable output
- Needs more guardrails ("do NOT" lists)
- May need revision cycles

**Best for:** Complex logic, unique problems, when optimal solution unclear

### Choosing the Right Level

| Situation | Recommended Level |
|-----------|------------------|
| Standard CRUD handlers | Prescriptive |
| Boilerplate/scaffolding | Prescriptive |
| Complex business logic | Abstract |
| Performance-critical code | Abstract (let agent optimize) |
| Test utilities | Prescriptive |
| API clients | Abstract with examples |

## Lessons Learned

### Auth Package Coordination Retrospective (January 2026)

See: `docs/retrospectives/auth-package-coordination-20260124.md`

**Key findings from coordinating 3 parallel Haiku agents:**

| Issue | Root Cause | Fix |
|-------|------------|-----|
| Agents created stubs | Ignored dependency declarations | Enforce sequential execution for dependent issues |
| Duplicate `mockStorage` | Same package, parallel branches | Use shared test utilities or sequential merges |
| Branch push 403 errors | Wrong session ID in branch name | Coordinator passes explicit branch names |
| 3 wasted CI runs | Reactive conflict resolution | Proactive dependency analysis before spawning |

**Recommended approach: Hybrid execution**
```
Independent issues (no deps, no file conflicts) → Parallel
Dependent issues → Sequential (after deps merge)
```

**Anti-patterns to avoid:**
1. ❌ Spawning all agents in parallel when dependencies exist
2. ❌ Using placeholders like `[SESSION_ID]` instead of explicit values
3. ❌ Allowing agents to create stub implementations
4. ❌ Creating PR before CI passes

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

### Critical Lesson: Worktree Isolation Required for Parallel Execution

**Discovered:** January 2026, storage layer implementation

**Problem with shared checkout:**
- Sub-agents working in `/home/user/project` could see each other's uncommitted files
- Led to scope violations (agent #39 created files meant for agent #40)
- 10+ CI fix commits across PRs
- Required manual cleanup

**Solution with worktrees:**
- Each agent in `/home/user/project-wt-{issue}/`
- Complete file isolation
- Zero interference
- 100% CI pass rate on PR creation

**Results comparison:**

| Metric | Shared Checkout | Git Worktree |
|--------|----------------|--------------|
| Scope adherence | ❌ Failed | ✅ Perfect |
| CI on first push | 0/3 PRs | 3/3 PRs |
| Fix commits needed | 10+ | 0 |
| File interference | Yes | None |

**Recommendation:** **Always use worktrees for parallel sub-agents.** The setup overhead (~5 lines in issue) prevents hours of debugging.

### PR Check Verification

**Critical addition:** Sub-agents must verify PR checks pass before declaring success.

**Before:**
```bash
7. Create PR to main
8. Wait for CI to pass
9. Post completion summary
```

**After:**
```bash
7. Create PR to main
8. Wait for CI to pass
9. Verify PR shows all checks passing: gh pr checks [PR#] --watch
10. Post completion summary (only after PR is fully green)
```

**Why this matters:**
- CI can pass on push but PR checks may still be pending
- Coordinator needs confirmation that PR is ready to merge
- Prevents premature "success" reports

## Token Usage Tracking

Sub-agents should report in this format:
```
## Token Usage
- Input: ~X,XXX tokens
- Output: ~X,XXX tokens
- Total: ~X,XXX tokens
```

**Real-world data from bunny-api-proxy implementation:**

| Task Type | Output Tokens | Total Tokens | Haiku Cost |
|-----------|---------------|--------------|------------|
| Types/structs (#20) | 6,961 | 1.4M | $0.22 |
| Client struct (#21) | 4,564 - 9,721 | 1.1M - 3.3M | $0.15 - $0.37 |
| API method + tests (#22-25) | 21K - 58K | 10M - 33M | $1.03 - $3.23 |
| Merge conflict resolution | 15K - 22K | 1.5M - 3.4M | $0.34 - $0.49 |
| Coverage fix (#28) | 7,347 | 1.3M | $0.22 |

**Note:** Total tokens are high due to cache reads (context), but cache reads are discounted 90%. Actual costs are very low.

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

## Token Usage Extraction

A script is available to extract token usage from session transcripts:

```bash
# Full report with all sessions
./scripts/extract-tokens.py

# Summary table only
./scripts/extract-tokens.py --summary

# Single transcript file
./scripts/extract-tokens.py ~/.claude/projects/.../subagents/agent-XXXXX.jsonl
```

**Sample output:**
```
SUMMARY: SUBAGENT TOKEN USAGE
==========================================================================================
Issue    Agent      Output     Total        Haiku $    Opus $     Saved
------------------------------------------------------------------------------------------
#20      adff3d6    6,961      1,392,743    $0.22      $21.31     $21.09
#21      a07629a    4,564      1,129,162    $0.15      $17.21     $17.06
...
------------------------------------------------------------------------------------------
TOTAL                                       $17.84     $2495.72   $2477.87 (99%)
```

**Transcript locations:**
- Main sessions: `~/.claude/projects/{project}/`
- Subagents: `~/.claude/projects/{project}/{session-id}/subagents/`

The script parses JSONL files and extracts:
- `input_tokens` - direct input tokens
- `output_tokens` - output tokens
- `cache_creation_input_tokens` - tokens written to cache (25% premium)
- `cache_read_input_tokens` - tokens read from cache (90% discount)

**Cost calculation:**
- Haiku: $0.80/1M input, $4.00/1M output, $0.08/1M cache read, $1.00/1M cache write
- Opus: $15.00/1M input, $75.00/1M output, $1.50/1M cache read, $18.75/1M cache write

## Coordinator Cost Optimization

**Reality check:** In the bunny-api-proxy project, the coordinator (Opus) cost $2,473 while subagents (Haiku) cost only $18. The coordinator dominates total project cost.

| Component | Cost | % of Total |
|-----------|------|------------|
| Coordinator (Opus) | $2,473 | 99.3% |
| Subagents (Haiku) | $18 | 0.7% |
| **Total** | **$2,491** | |

### Why Coordinator Costs Dominate

1. **Long-running sessions** - Coordinator stays active across multiple subagent tasks
2. **Context accumulation** - Each turn adds to the conversation history
3. **Opus pricing** - 19x more expensive than Haiku for input, 19x for output
4. **Design overhead** - Specs, reviews, orchestration all happen in Opus

### Optimization Strategies

| Strategy | Potential Savings | Trade-off |
|----------|------------------|-----------|
| **Batch task creation** | High | Create all issues in one pass before spawning any subagents |
| **Shorter sessions** | High | End session after setup, start new session for reviews |
| **Delegate more to Haiku** | Medium | Use Haiku for exploration, research, initial design |
| **Prescriptive specs** | Medium | Less back-and-forth during implementation |
| **Parallel subagents** | Low | Reduces coordinator wait time, not token usage |

### When to Use Each Model

| Task Type | Recommended Model | Rationale |
|-----------|------------------|-----------|
| Architecture decisions | Opus | Needs deep reasoning |
| Issue/spec creation | Opus | Quality specs save subagent costs |
| Code exploration | Haiku | Cheaper for search/read tasks |
| Implementation | Haiku | 99% savings on coding |
| Code review | Opus | Catches subtle issues |
| PR merge | Opus (minimal) | Simple commands, low cost |

### Session Management Tips

1. **Plan before starting** - Have a clear task list before engaging the coordinator
2. **Batch similar work** - Create all related issues at once, spawn subagents together
3. **Use concise prompts** - The coordinator already knows context; don't repeat
4. **End sessions cleanly** - Don't leave sessions running idle
5. **Consider session splits** - Design phase → end session → Implementation phase → end session → Review phase
