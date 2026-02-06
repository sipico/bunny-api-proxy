# Issue #225 Documentation Index

All files extracted and documented for **Issue #225: Bunny.net API Timestamp Format Inconsistencies**

## 📋 Quick Start

**For Bunny.net Support Team:**
1. Read: `bunny-api-timestamp-formats.md` (comprehensive analysis)
2. Use: `email-template-for-support.md` (ready-to-send email)
3. Reference: `timestamp-examples-table.md` (data tables)

**For Quick Understanding:**
- Start here: `QUICK-REFERENCE.md` (1-page summary)
- Or here: `README.md` (overview of all files)

## 📁 File Organization

### Documentation Files (Read These)

| File | Purpose | Lines | When to Use |
|------|---------|-------|-------------|
| `README.md` | Overview and file guide | 107 | Start here for context |
| `QUICK-REFERENCE.md` | 1-page summary with examples | 112 | Quick understanding |
| `bunny-api-timestamp-formats.md` | Complete analysis for support | 308 | **Send to bunny.net** |
| `email-template-for-support.md` | Ready-to-send email draft | 99 | Copy/paste to support |
| `timestamp-examples-table.md` | Data tables and statistics | 122 | Reference specific examples |
| `INDEX.md` | This file - navigation guide | - | Find what you need |

### Raw Log Files (Evidence)

| File | Content | Lines | Size |
|------|---------|-------|------|
| `raw-logs-post-create.txt` | POST /dnszone requests/responses | 60 | 26KB |
| `raw-logs-get-list.txt` | GET /dnszone requests/responses | 60 | 12KB |
| `raw-logs-get-by-id.txt` | GET /dnszone/{id} requests/responses | 47 | 9.4KB |
| `full-proxy.log` | Complete proxy log from CI run | 6800+ | 358KB |

## 🎯 Use Cases

### Scenario 1: "I need to email bunny.net support"
1. Open `email-template-for-support.md`
2. Customize the greeting and signature
3. Attach `bunny-api-timestamp-formats.md`
4. Send!

### Scenario 2: "I need a quick summary for my team"
1. Share `QUICK-REFERENCE.md`
2. Point them to `README.md` for more context

### Scenario 3: "I need specific log examples"
1. Check `timestamp-examples-table.md` for data tables
2. Reference raw log files for actual log lines
3. Use `full-proxy.log` for complete context

### Scenario 4: "I need to understand the full issue"
1. Start with `README.md`
2. Read `bunny-api-timestamp-formats.md`
3. Review `timestamp-examples-table.md` for patterns

## 🔍 Key Findings Summary

**Three Different Timestamp Formats:**

```
POST /dnszone    → "2026-02-06T09:27:36.7357793Z"  (7 digits + Z)
GET /dnszone     → "2026-02-06T09:27:36"           (no sub-seconds, no Z)
GET /dnszone/123 → "2026-02-06T09:27:37"           (no sub-seconds, no Z)
```

**Impact:**
- ❌ Sub-second precision lost after zone creation
- ❌ Timezone indicator (`Z`) missing on GET endpoints
- ❌ API consumers must handle 3 different formats
- ❌ Mock servers must replicate per-endpoint behavior

## 📊 Statistics

From analysis of logs in this directory:
- **20+ zones created** with sub-second precision
- **20+ zones retrieved** with precision truncated
- **100% consistency** within each endpoint type
- **0 exceptions** to the format pattern
- **6-7 digits** of sub-second precision on POST
- **0 digits** of sub-second precision on GET

## 🔗 External Links

- **GitHub Issue:** https://github.com/sipico/bunny-api-proxy/issues/225
- **CI Run:** https://github.com/sipico/bunny-api-proxy/actions/runs/21745257924
- **Job:** E2E Tests (Real Bunny.net API) #62730324295
- **Artifact:** `e2e-logs-real-api` (contains `proxy.log`)

## 📅 Timeline

- **2026-02-06 09:27 UTC** - E2E tests run against real Bunny API
- **2026-02-06 09:28 UTC** - Logs captured and artifacts uploaded
- **2026-02-06** - Issue #225 opened
- **2026-02-06 10:31-10:33 UTC** - Logs extracted and documented in this directory

## ✅ Completeness Checklist

All requested data extracted:
- ✅ Full request to real bunny API endpoints
- ✅ Full response from bunny API endpoints
- ✅ Examples showing POST /dnszone format (sub-second + Z)
- ✅ Examples showing GET /dnszone format (seconds only)
- ✅ Examples showing GET /dnszone/{id} format (seconds only)
- ✅ Comparison showing differences between endpoints
- ✅ Multiple examples (20+ zones documented)
- ✅ Raw logs preserved for reference
- ✅ Formatted documentation for support team
- ✅ Ready-to-send email template
- ✅ Quick reference guide
- ✅ Data tables and statistics

## 🚀 Next Steps

1. **Review** the documentation in this directory
2. **Send** `bunny-api-timestamp-formats.md` to bunny.net support using the email template
3. **Wait** for bunny.net's response on whether this is intentional
4. **Update** mockbunny to match real API behavior (per Issue #225)
5. **Document** the resolution in Issue #225

## 📝 Notes

- All timestamps in logs are UTC
- The proxy normalizes timestamps by adding `Z` when missing
- The mockbunny currently uses a single format for all endpoints (incorrect)
- Issue #225 tracks fixing mockbunny to match reality

## 🔐 Access

All files in this directory are:
- Part of the bunny-api-proxy repository
- Located at: `.claude/dev/issues/225/`
- Committed to git: [pending]
- Public: Yes (open source project)

---

**Generated:** 2026-02-06
**For Issue:** #225
**Run:** 21745257924
**Purpose:** Forward to bunny.net support team
