# Mock vs Real API Comparison - Coordination Plan

**Issues:** #215 - #226 (12 issues)
**Goal:** Fix mockbunny server to better match real Bunny.net API behavior

## Issue Validation Summary

All 12 issues were validated against the current codebase (2026-02-06). Line numbers
and code snippets are accurate. Several issues were missing test file updates (corrected
in the per-issue plans posted as comments).

| Issue | Title | Validated | Corrections |
|-------|-------|-----------|-------------|
| #215 | ErrorKey dot vs underscore | OK | Missing: test update (handlers_test.go:600) |
| #216 | Validation field priority | OK | None |
| #217 | SoaEmail wrong value | OK | Missing: test updates (handlers_test.go:561, server_test.go:165) |
| #218 | NameserversNextCheck null | OK | None |
| #219 | LoggingIPAnonymization default | OK | None |
| #220 | Content-Type missing charset | OK | None |
| #221 | JSON trailing newline | OK | None |
| #222 | Error messages trailing \r | OK | Recommend close as "won't fix" |
| #223 | Zone name uniqueness | OK | None |
| #224 | Nameserver values wrong | OK | Missing: test updates (3 test files) |
| #225 | Timestamp format wrong | OK | Missing: test updates (types_test.go) |
| #226 | JSON field ordering | OK | None |

## File Conflict Analysis

Nearly all issues touch `handlers.go` and/or `server.go`. This table shows which
files each issue modifies:

| Issue | handlers.go | server.go | types.go | admin.go | testenv.go | Dockerfile |
|-------|:-----------:|:---------:|:--------:|:--------:|:----------:|:----------:|
| #215  | L186,190,246 | | | | | |
| #216  | L183-192 | | | | | |
| #217  | L276 | L137 | | | | |
| #218  | ~L265-290 | ~L126 | | | | |
| #219  | ~L277 | ~L138 | | | | |
| #220  | L72,97,226,286,322 | L93,101 | | L48,99,134 | | |
| #221  | L74,99,229,289,325 | | | L51,102,136 | | |
| #222  | (none) | | | | | |
| #223  | | | | | L373-381 | L4 |
| #224  | L274-275 | L135-136 | | | | |
| #225  | | | L23-30 | | | |
| #226  | L65-70 | | L120-125 | | | |

### Test Files Modified

| Issue | handlers_test.go | server_test.go | types_test.go |
|-------|:----------------:|:--------------:|:-------------:|
| #215  | L600-601 | | |
| #216  | (none) | | |
| #217  | L561-562 | L165-166 | |
| #218  | | | |
| #219  | | | |
| #220  | | | |
| #221  | | | |
| #224  | L555-559 | L168-172 | L85-86,103-104 |
| #225  | | | L166,252 + roundtrip test |
| #226  | | | |

## Execution Strategy

### Why Sequential (Mostly)

Almost all issues modify `handlers.go`. While they touch different line ranges,
the high density of changes makes parallel execution risky for merge conflicts.
The recommended approach is **batched sequential** with some parallel opportunities.

### Batch 1 - Independent (PARALLEL)

These issues have ZERO file conflicts with each other or with later batches:

| Issue | Files | Rationale |
|-------|-------|-----------|
| **#222** | (none) | Close as "won't fix" - no code changes needed |
| **#223** | testenv.go, Dockerfile.testrunner | Completely independent package |
| **#225** | types.go (MarshalJSON only) | Isolated to timestamp serialization |

**#226** could also go here (types.go ListZonesResponse + handlers.go L65-70), but since
it touches handlers.go, safer to wait until after validation block changes merge.

### Batch 2 - Validation Block (SEQUENTIAL)

Both touch the validation block in `handleAddRecord` (handlers.go L183-192):

1. **#215** first - Change "validation.error" to "validation_error" (3 locations + 1 test)
2. **#216** second - Swap Value/Name validation order

**Why sequential:** Lines 183-192 overlap directly. #215 changes the string values,
#216 swaps the if-block order.

### Batch 3 - Zone Creation Defaults (SEQUENTIAL or COMBINED)

All modify the zone struct initializer in `handleCreateZone` (L265-290) and `AddZone` (L117-146):

1. **#217** - SoaEmail: "admin@domain" -> "hostmaster@bunny.net"
2. **#224** - Nameservers: ns1/ns2 -> kiki/coco
3. **#218** - Add NameserversNextCheck timestamp
4. **#219** - Set LoggingIPAnonymization: true

**Recommendation:** These 4 issues are simple value changes in the same struct literal.
Consider combining into a single subagent task for efficiency. If kept separate, run
sequentially to avoid merge conflicts.

### Batch 4 - Response Format (SEQUENTIAL or COMBINED)

Both touch the same lines (Content-Type + json.Encode) across the same files:

1. **#220** - Add charset=utf-8 to Content-Type
2. **#221** - Replace json.NewEncoder with json.Marshal (remove trailing newline)

**Recommendation:** Combine into a single task. The `writeJSON` helper from #221
naturally incorporates the charset fix from #220. Doing them separately would require
changing the same lines twice.

### Batch 5 - Struct Ordering (INDEPENDENT)

- **#226** - Reorder ListZonesResponse fields

Can run anytime after Batch 2 (touches handlers.go L65-70, far from other changes).

### Execution Timeline

```
Batch 1: #222 + #223 + #225  (parallel)
         ↓ merge all
Batch 2: #215 → #216         (sequential)
         ↓ merge all
Batch 3: #217 → #224 → #218 → #219  (sequential, or combined as 1 task)
         ↓ merge all
Batch 4: #220 → #221         (sequential, or combined as 1 task)
         ↓ merge all
Batch 5: #226                 (independent)
```

**Optimized timeline (combining related issues):**

```
Batch 1: #222 (close) + #223 + #225          (parallel)
         ↓ merge
Batch 2: #215 + #216 (combined)              (single task)
         ↓ merge
Batch 3: #217 + #218 + #219 + #224 (combined) (single task)
         ↓ merge
Batch 4: #220 + #221 (combined)              (single task)
         ↓ merge
Batch 5: #226                                 (single task)
```

This reduces from 12 separate subagent tasks to 7 (with 3 parallel in batch 1).

## Subagent Session ID

All subagent branches must use the coordinator's session ID: `OhwYT`

Branch naming: `claude/issue-{NUM}-OhwYT`
Worktree path: `/home/user/bunny-api-proxy-wt-{NUM}`
