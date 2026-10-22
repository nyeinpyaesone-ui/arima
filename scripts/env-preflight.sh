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
    [[ "$ARIMA_OIDC_ISSUER" == https://* ]] || {
      echo "ERROR: OIDC issuer must use HTTPS." >&2
      exit 1
    }
    echo "OIDC values passed presence/syntax checks only; runtime OIDC enforcement is not implemented by this scaffold."
    ;;
  *)
    echo "ERROR: ARIMA_ENV must be development, staging, or production." >&2
    exit 1
    ;;
esac

# PostgreSQL is deliberately not required here: current application code does
# not implement persistence or migrations, and this script must not imply it does.
go mod verify
test -z "$(gofmt -l ./cmd ./internal)"
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.1.4 ./...
go test -race ./...
CGO_ENABLED=0 GOOS=linux go build -trimpath -o /tmp/arima-api ./cmd/erp-api
echo "Preflight passed. No deployment or migration was performed."
