# API Exploration Branch

**Branch:** `claude/exploration-api-SHP62`

This branch is dedicated to iterative API exploration. It contains the `explore-api.yml` workflow that can be modified and run multiple times without triggering the full CI pipeline.

---

## Purpose

Explore the real bunny.net API behavior using simple curl commands:
- Test domain availability checking
- Test DNS record scanning
- Test DNSSEC operations
- Discover actual response formats

---

## Why a Separate Branch?

✅ **Faster iteration** - No CI runs on workflow changes
✅ **Independent development** - Don't pollute main/feature branches
✅ **Safe experimentation** - Can run multiple times
✅ **Easy cleanup** - Delete branch when done

---

## How to Use

### 1. Switch to Exploration Branch

```bash
git checkout claude/exploration-api-SHP62
```

### 2. Run the Exploration Workflow

**Option A: GitHub UI**
1. Go to **Actions** tab
2. Select **"Explore bunny.net API"** workflow
3. Click **"Run workflow"**
4. Select branch: `claude/exploration-api-SHP62`
5. Choose exploration step (e.g., `all`, `zones`, `scanning`)
6. Click **"Run workflow"**

**Option B: gh CLI**
```bash
gh workflow run explore-api.yml \
  --repo sipico/bunny-api-proxy \
  --ref claude/exploration-api-SHP62
```

### 3. Review Results

**View logs:**
```bash
gh run list --workflow=explore-api.yml --branch=claude/exploration-api-SHP62
gh run view <run-id> --log
```

**Download artifacts:**
```bash
gh run download <run-id>
# or via UI: Actions → Run → Artifacts section
```

### 4. Iterate

Based on what you learned:

1. **Edit the workflow:**
   ```bash
   vim .github/workflows/explore-api.yml
   # Add new curl commands
   # Test different domains
   # Explore new endpoints
   ```

2. **Commit changes:**
   ```bash
   git add .github/workflows/explore-api.yml
   git commit -m "Explore: Test XYZ endpoint"
   git push
   ```

3. **Run again:**
   ```bash
   gh workflow run explore-api.yml \
     --repo sipico/bunny-api-proxy \
     --ref claude/exploration-api-SHP62
   ```

4. **Repeat** until you have all the information you need

---

## Example Iteration Flow

```bash
# 1. Initial exploration - see what exists
gh workflow run explore-api.yml --ref claude/exploration-api-SHP62

# 2. Review logs - notice amazon.com zone creation succeeds
gh run view <run-id> --log

# 3. Add new step to scan amazon.com
vim .github/workflows/explore-api.yml
# Add: curl POST /dnszone/$AMAZON_ZONE_ID/recheckdns

# 4. Commit and run again
git add .github/workflows/explore-api.yml
git commit -m "Explore: Scan amazon.com DNS records"
git push
gh workflow run explore-api.yml --ref claude/exploration-api-SHP62

# 5. Review new results
gh run view <new-run-id> --log

# 6. Repeat...
```

---

## Current Exploration Steps

See `.github/workflows/README-explore-api.md` for detailed documentation.

**Summary:**
- **Init:** Clear all zones from account
- **Zones:** Create test zone, try amazon.com
- **Availability:** Check various domains
- **DNSSEC:** Enable/disable, inspect DS records
- **Scanning:** Trigger scan, poll results
- **Cleanup:** Delete all zones, verify empty

---

## Adding New Exploration Steps

Edit `.github/workflows/explore-api.yml`:

```yaml
- name: Explore - Your new test
  if: github.event.inputs.step == 'all' || github.event.inputs.step == 'your-step'
  run: |
    echo "=========================================="
    echo "EXPLORE: What you're testing"
    echo "=========================================="

    curl -v -X POST \
      -H "AccessKey: ${{ secrets.BUNNY_API_KEY }}" \
      -H "Content-Type: application/json" \
      -d '{"Domain": "test.xyz"}' \
      "$BUNNY_API_URL/dnszone/endpoint" \
      2>&1 | tee your-test.log

    echo ""
    echo "Pretty-printed:"
    curl -s -X POST \
      -H "AccessKey: ${{ secrets.BUNNY_API_KEY }}" \
      -H "Content-Type: application/json" \
      -d '{"Domain": "test.xyz"}' \
      "$BUNNY_API_URL/dnszone/endpoint" \
      | jq '.'
```

Also add your step to the input options:
```yaml
on:
  workflow_dispatch:
    inputs:
      step:
        options:
          - all
          - your-step  # Add here
```

---

## CI Behavior

✅ **Changes to `explore-api.yml` on this branch DO NOT trigger CI**
✅ **Changes to other files still trigger CI normally**
✅ **Workflow runs don't affect CI at all**

This is configured via `paths-ignore` in `.github/workflows/ci.yml`:
```yaml
paths-ignore:
  - '.github/workflows/explore-api.yml'
```

---

## Documenting Findings

As you discover API behaviors, document them:

**Create:** `.claude/dev/REAL_API_RESPONSES.md`

```markdown
# Real bunny.net API Responses

## Zone Creation

### Create amazon.com zone
**Request:**
POST /dnszone
{"Domain": "amazon.com"}

**Response:** 201 Created
{
  "Id": 67890,
  "Domain": "amazon.com",
  "Created": "2026-02-08T12:34:56Z"
}

**Surprise:** bunny.net allows adding amazon.com as a zone even though we don't own it!
...
```

---

## When You're Done

### Option 1: Merge Findings Back

If you improved the workflow:
```bash
# Switch back to your feature branch
git checkout claude/add-e2e-endpoint-tests-SHP62

# Cherry-pick specific commits
git cherry-pick <commit-hash>

# Or merge entire exploration branch
git merge claude/exploration-api-SHP62
```

### Option 2: Delete Exploration Branch

If exploration is complete:
```bash
git push origin --delete claude/exploration-api-SHP62
git branch -d claude/exploration-api-SHP62
```

---

## Tips

### View All Workflow Runs
```bash
gh run list --workflow=explore-api.yml --branch=claude/exploration-api-SHP62 --limit 10
```

### Cancel a Running Workflow
```bash
gh run cancel <run-id>
```

### Re-run a Previous Workflow
```bash
gh run rerun <run-id>
```

### Watch Logs in Real-Time
```bash
gh run watch <run-id>
```

### Compare Responses Across Runs
```bash
# Download artifacts from multiple runs
gh run download <run-id-1> -D exploration-run-1
gh run download <run-id-2> -D exploration-run-2

# Compare responses
diff exploration-run-1/create-test-zone.log exploration-run-2/create-test-zone.log
```

---

## Troubleshooting

**"Workflow not found":**
- Make sure you're on the right branch: `git checkout claude/exploration-api-SHP62`
- Verify workflow file exists: `ls .github/workflows/explore-api.yml`
- Push the branch: `git push -u origin claude/exploration-api-SHP62`

**"403 Forbidden" on push:**
- Branch name must start with `claude/` and end with session ID
- Current valid name: `claude/exploration-api-SHP62`

**Workflow fails with cleanup errors:**
- The cleanup step has `if: always()` so it runs even on failure
- Check logs to see which zones couldn't be deleted
- May need to manually delete via bunny.net dashboard

**API key not working:**
- Verify `BUNNY_API_KEY` secret is set in repository settings
- Check key has necessary permissions (DNS zone management)

---

## Summary

This branch provides a safe, iterative environment for exploring the bunny.net API without affecting your main development workflow. Use it to discover real API behaviors, test edge cases, and gather response formats for implementing proper e2e tests.

**Key Points:**
- ✅ Changes to workflow don't trigger CI
- ✅ Can run workflow multiple times
- ✅ Logs everything for analysis
- ✅ Automatic cleanup
- ✅ Easy to iterate
