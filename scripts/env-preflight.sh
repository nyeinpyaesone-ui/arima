#!/usr/bin/env bash
set -euo pipefail

# Safe local preflight. Does not load dotenv files or print secret values.
command -v go >/dev/null || { echo "ERROR: Go is required." >&2; exit 1; }
go version

if [[ "${ARIMA_ENV:-development}" == "production" ]]; then
  : "${ARIMA_OIDC_ISSUER:?ARIMA_OIDC_ISSUER is required in production}"
  : "${ARIMA_OIDC_AUDIENCE:?ARIMA_OIDC_AUDIENCE is required in production}"
  : "${ARIMA_DATABASE_URL:?ARIMA_DATABASE_URL is required in production}"
  [[ "$ARIMA_OIDC_ISSUER" == https://* ]] || { echo "ERROR: OIDC issuer must use HTTPS." >&2; exit 1; }
  [[ "$ARIMA_DATABASE_URL" == postgres://* || "$ARIMA_DATABASE_URL" == postgresql://* ]] || {
    echo "ERROR: ARIMA_DATABASE_URL must be a PostgreSQL DSN." >&2; exit 1;
  }
fi

go mod verify
test -z "$(gofmt -l ./cmd ./internal)"
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
go test -race ./...
CGO_ENABLED=0 GOOS=linux go build -trimpath -o /tmp/arima-api ./cmd/erp-api
echo "Preflight passed. No deployment or migration was performed."
