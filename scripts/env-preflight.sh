#!/usr/bin/env bash
set -euo pipefail

# Safe local preflight. Does not load dotenv files or print secret values.
command -v go >/dev/null || { echo "ERROR: Go is required." >&2; exit 1; }
go version

target="${ARIMA_ENV:-development}"
case "$target" in
  development)
    ;;
  staging|production)
    : "${ARIMA_OIDC_ISSUER:?ARIMA_OIDC_ISSUER is required for staging/production preflight}"
    : "${ARIMA_OIDC_AUDIENCE:?ARIMA_OIDC_AUDIENCE is required for staging/production preflight}"
    : "${ARIMA_DATABASE_URL:?ARIMA_DATABASE_URL is required for staging/production preflight}"
    [[ "$ARIMA_OIDC_ISSUER" == https://* ]] || {
      echo "ERROR: OIDC issuer must use HTTPS." >&2
      exit 1
    }
    [[ "$ARIMA_DATABASE_URL" == postgres://* || "$ARIMA_DATABASE_URL" == postgresql://* ]] || {
      echo "ERROR: production database URL must use postgres:// or postgresql://." >&2
      exit 1
    }
    [[ "$ARIMA_DATABASE_URL" == *sslmode=verify-full* ]] || {
      echo "ERROR: production database URL must use sslmode=verify-full." >&2
      exit 1
    }
    echo "OIDC and PostgreSQL values passed presence/syntax checks only; no remote identity or database connection is made."
    ;;
  *)
    echo "ERROR: ARIMA_ENV must be development, staging, or production." >&2
    exit 1
    ;;
esac

go mod verify
files="$(gofmt -l ./cmd ./internal)"
if [[ -n "$files" ]]; then
  printf 'Go files need gofmt:\n%s\n' "$files" >&2
  exit 1
fi
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
# Full PostgreSQL integration coverage runs in GitHub Actions CI.
go test -race ./...
CGO_ENABLED=0 GOOS=linux go build -trimpath -o /tmp/arima-api ./cmd/erp-api
echo "Preflight passed. No deployment or migration was performed."
