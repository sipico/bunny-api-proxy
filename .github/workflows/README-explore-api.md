# API Exploration Workflow

This workflow allows iterative exploration of the bunny.net API using simple curl commands.

## Purpose

Discover real API behavior for endpoints that are hard to test with fake domains:
- Domain availability checking
- DNS record scanning
- DNSSEC operations
- Certificate issuance

## How to Use

### 1. Trigger the Workflow

Go to **Actions** → **Explore bunny.net API** → **Run workflow**

Choose which exploration step to run:
- `all` - Run full exploration suite (recommended for first run)
- `cleanup-only` - Just clear all zones from account
- `zones` - Test zone creation (including amazon.com)
- `availability` - Test domain availability checks
- `dnssec` - Test DNSSEC enable/disable
- `scanning` - Test DNS record scanning

### 2. Review the Logs

The workflow logs show:
- **Verbose curl output** (`-v` flag) with full HTTP details
- **Pretty-printed JSON** responses (via `jq`)
- **Success/failure** for each operation
- **Timing information**

Look for:
- HTTP status codes (200, 201, 400, 404, etc.)
- Response body structure
- Error messages and formats
- Headers (Content-Type, rate limits, etc.)

### 3. Download Artifacts

After the run completes:
1. Go to the workflow run page
2. Scroll to **Artifacts** section
3. Download `api-exploration-logs`
4. Contains all `.log`, `.json`, and `.txt` files

### 4. Iterate

Based on what you learned:
1. Add new exploration steps to `explore-api.yml`
2. Test edge cases
3. Try different domains or parameters
4. Document findings

## Current Exploration Steps

### Initialization (Always Runs)
- Lists all existing zones
- Deletes all zones (cleanup from previous runs)
- Verifies account is empty

### Zone Operations
- ✅ Create test zone (`test-explore-bap.xyz`)
- ✅ Try to create `amazon.com` zone (do we get an error or success?)

### Availability Checks
- ✅ Check `amazon.com` (expect: not available)
- ✅ Check `google.com` (expect: not available)
- ✅ Check `definitely-not-registered-12345.xyz` (expect: available)
- ✅ Check our test zone (expect: not available? or available?)

### DNSSEC Operations
- ✅ Enable DNSSEC on test zone
- ✅ Get zone details to see DNSSEC status and DS records

### DNS Scanning
- ✅ Trigger scan on fake domain (test-explore-bap.xyz)
- ✅ Poll for scan results
- 🔜 TODO: Try scanning amazon.com (if we can add it as a zone)

### Cleanup (Always Runs)
- Lists all zones after exploration
- Deletes all zones
- Verifies account is empty

## Adding New Exploration Steps

Edit `.github/workflows/explore-api.yml` and add a new step:

```yaml
- name: Explore - Your new test
  if: github.event.inputs.step == 'all' || github.event.inputs.step == 'your-category'
  run: |
    echo "=========================================="
    echo "EXPLORE: Description of what you're testing"
    echo "=========================================="

    curl -v -X POST \
      -H "AccessKey: ${{ secrets.BUNNY_API_KEY }}" \
      -H "Content-Type: application/json" \
      -d '{"YourData": "here"}' \
      "$BUNNY_API_URL/dnszone/endpoint" \
      2>&1 | tee your-test.log

    echo ""
    echo "Pretty-printed:"
    curl -s -X POST \
      -H "AccessKey: ${{ secrets.BUNNY_API_KEY }}" \
      -H "Content-Type: application/json" \
      -d '{"YourData": "here"}' \
      "$BUNNY_API_URL/dnszone/endpoint" \
      | jq '.'
```

Then run the workflow again and review the new logs.

## Tips

### Verbose Output
The `-v` flag shows full HTTP exchange:
```
> POST /dnszone HTTP/2
> Host: api.bunny.net
> AccessKey: ***
< HTTP/2 201
< content-type: application/json
< {"Id": 12345, "Domain": "test.xyz"}
```

### Pretty Printing
Use `jq '.'` to format JSON:
```bash
curl -s ... | jq '.'
# or save to file:
curl -s ... | jq '.' > response.json
```

### Error Handling
If a curl command fails, the workflow continues (doesn't stop):
```bash
curl -s ... | jq '.' || echo "Request failed or returned non-JSON"
```

### Saving Data Between Steps
Use environment files to pass data:
```bash
ZONE_ID=$(jq -r '.Id' response.json)
echo "ZONE_ID=$ZONE_ID" >> $GITHUB_ENV
# Later steps can use: $ZONE_ID or ${{ env.ZONE_ID }}
```

## Next Steps After Exploration

Once you've gathered API responses:

1. **Document findings** in `.claude/dev/REAL_API_RESPONSES.md`
2. **Update mockbunny** with realistic response formats
3. **Write e2e tests** based on actual API behavior
4. **Update test expectations** if API differs from assumptions

## Security Notes

- ✅ API key is stored in GitHub Secrets (`BUNNY_API_KEY`)
- ✅ Key is never exposed in logs (GitHub masks secret values)
- ✅ Workflow can only be triggered manually (no automatic runs)
- ✅ Always runs cleanup to prevent orphaned zones

## Troubleshooting

**Workflow fails with "No zones to delete":**
- This is OK! Means the account was already empty

**Workflow fails on cleanup:**
- Check if zones were created successfully
- Review deletion logs for errors
- May need to manually delete zones via bunny.net dashboard

**curl shows 401 Unauthorized:**
- Check that `BUNNY_API_KEY` secret is set correctly
- Verify the API key has necessary permissions

**curl shows 429 Too Many Requests:**
- bunny.net rate limit hit
- Wait a few minutes and try again
- Consider adding `sleep` between requests
