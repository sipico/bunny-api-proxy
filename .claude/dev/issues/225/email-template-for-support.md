# Email Template for Bunny.net Support

---

**Subject:** DNS API Timestamp Format Inconsistencies Across Endpoints

---

**Body:**

Hello Bunny.net Support Team,

We're developing an API proxy for your DNS API and discovered that different endpoints return timestamps in different formats for the same data. This is causing challenges for our testing infrastructure.

## Issue Summary

The DNS API returns timestamps in **three different formats** depending on which endpoint is called:

| Endpoint | Example Timestamp | Format |
|----------|------------------|--------|
| POST /dnszone (create) | `2026-02-06T09:27:36.7357793Z` | Sub-second precision (7 digits) + Z |
| GET /dnszone (list) | `2026-02-06T09:27:36` | Seconds only, no Z |
| GET /dnszone/{id} (get) | `2026-02-06T09:27:37` | Seconds only, no Z |

## Concrete Example

When we create a zone via POST, the response includes:
```json
{
  "Id": 716775,
  "DateCreated": "2026-02-06T09:27:36.7357793Z"
}
```

However, when we retrieve the same zone via GET, it returns:
```json
{
  "Id": 716775,
  "DateCreated": "2026-02-06T09:27:36"
}
```

The sub-second precision and timezone indicator are lost.

## Detailed Evidence

We've extracted detailed logs from our e2e tests running against your production API on 2026-02-06. The complete analysis with request/response examples is available here:

**GitHub Repository:** https://github.com/sipico/bunny-api-proxy
**Issue:** https://github.com/sipico/bunny-api-proxy/issues/225
**Test Logs:** https://github.com/sipico/bunny-api-proxy/actions/runs/21745257924

We can also provide the detailed analysis document attached to this email if needed.

## Questions

1. **Is this timestamp format variance intentional?** We want to ensure our mock server accurately replicates your API's behavior.

2. **Why do creation endpoints include sub-second precision but retrieval endpoints truncate it?** This means clients cannot retrieve the exact creation time they received when creating a zone.

3. **Why do GET endpoints omit the `Z` timezone suffix?** This creates ambiguity about whether timestamps are UTC or local time. The RFC3339 standard uses `Z` to explicitly indicate UTC.

4. **Is there documentation for these format differences?** We couldn't find this documented in the API documentation.

5. **Is there a way to request consistent timestamp formatting?** For example, a query parameter or header to control timestamp precision and format across all endpoints.

## Impact

- **Data loss:** The exact creation timestamp cannot be retrieved after a zone is created
- **Timezone ambiguity:** Missing `Z` suffix makes it unclear whether timestamps represent UTC
- **Client complexity:** API consumers must handle three different timestamp formats
- **Testing challenges:** Mock servers need to replicate per-endpoint behavior

## Our Current Workaround

Our proxy currently normalizes timestamps by adding the `Z` suffix when missing (assuming all bunny.net timestamps are UTC). However, we cannot restore the lost sub-second precision from POST responses.

## Request

Could you please:
1. Confirm whether this behavior is intentional
2. Provide guidance on whether we should expect consistent formats in the future
3. Update the API documentation to clarify the timestamp format differences if this is permanent

Thank you for your time and assistance!

Best regards,
[Your Name]
bunny-api-proxy Project

---

## Attachments (Optional)

If the support system allows attachments, include:
- `bunny-api-timestamp-formats.md` (detailed analysis)
- `raw-logs-post-create.txt` (example POST logs)
- `raw-logs-get-list.txt` (example GET list logs)
- `raw-logs-get-by-id.txt` (example GET by-id logs)
