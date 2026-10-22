# Initial Operations Runbook

## Local verification
```sh
go version
make check
go run ./cmd/erp-api
```

## Probes
- Liveness: `GET /health/live`
- Readiness: `GET /health/ready`

The current readiness route reports process availability only. Before production, it must verify required dependencies with bounded timeouts and fail when required dependencies are unavailable.

## Incident collection
Capture deployment version, UTC timestamp, request/correlation ID, redacted logs, and recent change history. Never attach credentials, JWTs, database URLs, or unredacted customer records.

## Rollback
Deploy the last known-good immutable image or release. Database changes require a reviewed rollback/forward-fix plan. Do not automatically drop volumes or run destructive migrations during rollback.
