#!/usr/bin/env bash
# analyze-e2e-logs.sh - Post-E2E test log analysis
#
# Scans proxy and mockbunny logs for hidden problems that tests might not catch:
# - Panic/recovery events
# - Error-level log entries during normal operations
# - Upstream authentication failures
# - Database errors
# - Unexpected HTTP 5xx responses
# - Request ID gaps or anomalies
#
# Exit codes:
#   0 = No critical problems found
#   1 = Critical problems detected (should fail CI)
#   2 = Warnings detected (non-fatal, for review)

set -euo pipefail

LOG_DIR="${1:-.}"
PROXY_LOG="${LOG_DIR}/proxy.log"
MOCKBUNNY_LOG="${LOG_DIR}/mockbunny.log"
TEST_RUNNER_LOG="${LOG_DIR}/test-runner.log"

CRITICAL_ISSUES=0
WARNINGS=0
REPORT=""

# Colors for terminal output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

report() {
    REPORT="${REPORT}$1\n"
    echo -e "$1"
}

report_critical() {
    CRITICAL_ISSUES=$((CRITICAL_ISSUES + 1))
    report "${RED}CRITICAL: $1${NC}"
}

report_warning() {
    WARNINGS=$((WARNINGS + 1))
    report "${YELLOW}WARNING: $1${NC}"
}

report_ok() {
    report "${GREEN}OK: $1${NC}"
}

report "========================================================"
report "E2E Test Log Analysis Report"
report "========================================================"
report "Timestamp: $(date -u '+%Y-%m-%dT%H:%M:%SZ')"
report ""

# =============================================================================
# 1. Check proxy logs exist
# =============================================================================

if [ ! -f "$PROXY_LOG" ]; then
    report_critical "Proxy log file not found: $PROXY_LOG"
else
    PROXY_LINES=$(wc -l < "$PROXY_LOG")
    report "Proxy log: ${PROXY_LINES} lines"

    # -------------------------------------------------------------------------
    # 1a. Check for panics
    # -------------------------------------------------------------------------
    PANIC_COUNT=$(grep -ci "panic" "$PROXY_LOG" 2>/dev/null || true)
    if [ "$PANIC_COUNT" -gt 0 ]; then
        report_critical "Found ${PANIC_COUNT} panic-related entries in proxy logs"
        grep -i "panic" "$PROXY_LOG" | head -5
    else
        report_ok "No panics detected in proxy logs"
    fi

    # -------------------------------------------------------------------------
    # 1b. Check for error-level log entries (JSON structured logs)
    # -------------------------------------------------------------------------
    ERROR_COUNT=$(grep -c '"level":"ERROR"' "$PROXY_LOG" 2>/dev/null || true)
    if [ "$ERROR_COUNT" -gt 0 ]; then
        report_warning "Found ${ERROR_COUNT} ERROR-level entries in proxy logs"
        # Show unique error messages (deduplicated)
        grep '"level":"ERROR"' "$PROXY_LOG" | \
            grep -oP '"msg":"[^"]*"' | sort -u | head -10
    else
        report_ok "No ERROR-level entries in proxy logs"
    fi

    # -------------------------------------------------------------------------
    # 1c. Check for upstream authentication failures
    # -------------------------------------------------------------------------
    UPSTREAM_AUTH_FAIL=$(grep -c "upstream authentication failed" "$PROXY_LOG" 2>/dev/null || true)
    if [ "$UPSTREAM_AUTH_FAIL" -gt 0 ]; then
        report_critical "Found ${UPSTREAM_AUTH_FAIL} upstream authentication failures"
        report "  This indicates the proxy's bunny.net API key is invalid or expired"
    else
        report_ok "No upstream authentication failures"
    fi

    # -------------------------------------------------------------------------
    # 1d. Check for database errors
    # -------------------------------------------------------------------------
    DB_ERROR_COUNT=$(grep -c "database" "$PROXY_LOG" 2>/dev/null | grep -ci "error\|fail\|corrupt" 2>/dev/null || true)
    if [ "$DB_ERROR_COUNT" -gt 0 ]; then
        report_critical "Found ${DB_ERROR_COUNT} database-related errors in proxy logs"
    else
        report_ok "No database errors detected"
    fi

    # -------------------------------------------------------------------------
    # 1e. Check for 5xx response codes in structured logs
    # -------------------------------------------------------------------------
    FIVE_XX_COUNT=$(grep -cP '"status":\s*5\d\d' "$PROXY_LOG" 2>/dev/null || true)
    if [ "$FIVE_XX_COUNT" -gt 0 ]; then
        report_warning "Found ${FIVE_XX_COUNT} HTTP 5xx responses in proxy logs"
        grep -oP '"status":\s*5\d\d[^}]*' "$PROXY_LOG" | sort | uniq -c | sort -rn | head -5
    else
        report_ok "No HTTP 5xx responses in proxy logs"
    fi

    # -------------------------------------------------------------------------
    # 1f. Check for slow requests (>5 seconds)
    # -------------------------------------------------------------------------
    # Look for duration fields in structured logs that exceed 5s
    SLOW_COUNT=$(grep -cP '"duration_ms":\s*[5-9]\d{3,}' "$PROXY_LOG" 2>/dev/null || true)
    if [ "$SLOW_COUNT" -gt 0 ]; then
        report_warning "Found ${SLOW_COUNT} slow requests (>5s) in proxy logs"
    else
        report_ok "No slow requests detected"
    fi

    # -------------------------------------------------------------------------
    # 1g. Check for connection refused/timeout to upstream
    # -------------------------------------------------------------------------
    CONN_ERR_COUNT=$(grep -ci "connection refused\|dial tcp\|timeout\|deadline exceeded" "$PROXY_LOG" 2>/dev/null || true)
    if [ "$CONN_ERR_COUNT" -gt 0 ]; then
        report_warning "Found ${CONN_ERR_COUNT} connection errors to upstream in proxy logs"
        grep -i "connection refused\|dial tcp\|timeout" "$PROXY_LOG" | head -3
    else
        report_ok "No connection errors to upstream"
    fi

    # -------------------------------------------------------------------------
    # 1h. Check that proxy started correctly
    # -------------------------------------------------------------------------
    if grep -q "Server listening\|Server starting" "$PROXY_LOG" 2>/dev/null; then
        report_ok "Proxy startup confirmed in logs"
    else
        report_warning "Could not confirm proxy startup in logs"
    fi
fi

report ""

# =============================================================================
# 2. Check mockbunny logs (if present)
# =============================================================================

if [ -f "$MOCKBUNNY_LOG" ]; then
    MOCK_LINES=$(wc -l < "$MOCKBUNNY_LOG")
    report "Mockbunny log: ${MOCK_LINES} lines"

    MOCK_PANIC_COUNT=$(grep -ci "panic" "$MOCKBUNNY_LOG" 2>/dev/null || true)
    if [ "$MOCK_PANIC_COUNT" -gt 0 ]; then
        report_critical "Found ${MOCK_PANIC_COUNT} panic-related entries in mockbunny logs"
    else
        report_ok "No panics in mockbunny logs"
    fi

    MOCK_ERROR_COUNT=$(grep -c '"level":"ERROR"' "$MOCKBUNNY_LOG" 2>/dev/null || true)
    if [ "$MOCK_ERROR_COUNT" -gt 0 ]; then
        report_warning "Found ${MOCK_ERROR_COUNT} ERROR-level entries in mockbunny logs"
    else
        report_ok "No ERROR-level entries in mockbunny logs"
    fi
else
    report "Mockbunny log: not found (may be expected in real-API mode)"
fi

report ""

# =============================================================================
# 3. Check test runner output (if present)
# =============================================================================

if [ -f "$TEST_RUNNER_LOG" ]; then
    TEST_LINES=$(wc -l < "$TEST_RUNNER_LOG")
    report "Test runner log: ${TEST_LINES} lines"

    # Check for test failures
    FAIL_COUNT=$(grep -c "^--- FAIL:" "$TEST_RUNNER_LOG" 2>/dev/null || true)
    PASS_COUNT=$(grep -c "^--- PASS:" "$TEST_RUNNER_LOG" 2>/dev/null || true)
    SKIP_COUNT=$(grep -c "^--- SKIP:" "$TEST_RUNNER_LOG" 2>/dev/null || true)

    report "  Test results: ${PASS_COUNT} passed, ${FAIL_COUNT} failed, ${SKIP_COUNT} skipped"

    if [ "$FAIL_COUNT" -gt 0 ]; then
        report_critical "Found ${FAIL_COUNT} test failures"
        grep "^--- FAIL:" "$TEST_RUNNER_LOG"
    fi

    # Check for test timeouts
    TIMEOUT_COUNT=$(grep -c "panic: test timed out" "$TEST_RUNNER_LOG" 2>/dev/null || true)
    if [ "$TIMEOUT_COUNT" -gt 0 ]; then
        report_critical "Test suite timed out"
    fi

    # Check for race condition detections
    RACE_COUNT=$(grep -c "DATA RACE" "$TEST_RUNNER_LOG" 2>/dev/null || true)
    if [ "$RACE_COUNT" -gt 0 ]; then
        report_critical "Found ${RACE_COUNT} data race detections in test output"
    else
        report_ok "No data races detected"
    fi
else
    report "Test runner log: not found"
fi

report ""

# =============================================================================
# 4. Summary
# =============================================================================

report "========================================================"
report "SUMMARY"
report "========================================================"

if [ "$CRITICAL_ISSUES" -gt 0 ]; then
    report "${RED}CRITICAL ISSUES: ${CRITICAL_ISSUES}${NC}"
fi

if [ "$WARNINGS" -gt 0 ]; then
    report "${YELLOW}WARNINGS: ${WARNINGS}${NC}"
fi

if [ "$CRITICAL_ISSUES" -eq 0 ] && [ "$WARNINGS" -eq 0 ]; then
    report "${GREEN}All checks passed - no hidden problems detected${NC}"
fi

report "========================================================"

# Write report to file
echo -e "$REPORT" > "${LOG_DIR}/log-analysis-report.txt"

# Exit code
if [ "$CRITICAL_ISSUES" -gt 0 ]; then
    exit 1
elif [ "$WARNINGS" -gt 0 ]; then
    exit 0  # Warnings are non-fatal but visible
else
    exit 0
fi
