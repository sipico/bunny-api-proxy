# Release 2026.02.2

**Release Date:** February 4, 2026

## New Features

- **Prometheus metrics endpoint** (`GET /metrics`) - Monitor request counts, latencies, and auth failures
- **Request IDs for tracing** - `X-Request-ID` header in all responses
- **Two-level logging** - `LOG_LEVEL=debug` for full request/response details (sensitive data masked)

## Docker Images

```bash
docker pull ghcr.io/sipico/bunny-api-proxy:2026.02.2
docker pull ghcr.io/sipico/bunny-api-proxy:latest
```
