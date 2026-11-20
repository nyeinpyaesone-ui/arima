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
    : "${ARIMA_OUTBOX_DATABASE_URL:?ARIMA_OUTBOX_DATABASE_URL is required for staging/production preflight}"
    : "${ARIMA_OUTBOX_WEBHOOK_URL:?ARIMA_OUTBOX_WEBHOOK_URL is required for staging/production preflight}"
    : "${ARIMA_OUTBOX_WEBHOOK_TOKEN:?ARIMA_OUTBOX_WEBHOOK_TOKEN is required for staging/production preflight}"
    python3 - <<'PY'
import os
from urllib.parse import parse_qs, urlparse

issuer = urlparse(os.environ["ARIMA_OIDC_ISSUER"])
if issuer.scheme != "https" or not issuer.hostname:
    raise SystemExit("ARIMA_OIDC_ISSUER must be an absolute HTTPS URL")
if not os.environ["ARIMA_OIDC_AUDIENCE"].strip():
    raise SystemExit("ARIMA_OIDC_AUDIENCE must not be empty")

for name in ("ARIMA_DATABASE_URL", "ARIMA_OUTBOX_DATABASE_URL"):
    dsn = urlparse(os.environ[name])
    if dsn.scheme not in ("postgres", "postgresql") or not dsn.hostname:
        raise SystemExit(f"{name} must be a PostgreSQL URL with a hostname")
    if parse_qs(dsn.query).get("sslmode") != ["verify-full"]:
        raise SystemExit(f"{name} must use sslmode=verify-full")

webhook = urlparse(os.environ["ARIMA_OUTBOX_WEBHOOK_URL"])
if webhook.scheme != "https" or not webhook.hostname or webhook.username or webhook.password or webhook.fragment:
    raise SystemExit("ARIMA_OUTBOX_WEBHOOK_URL must be an absolute HTTPS URL without credentials or fragment")
if not os.environ["ARIMA_OUTBOX_WEBHOOK_TOKEN"].strip():
    raise SystemExit("ARIMA_OUTBOX_WEBHOOK_TOKEN must not be empty")

print("OIDC, database and webhook values passed syntax checks; secret values were not printed.")
PY
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
go test -race ./...
CGO_ENABLED=0 GOOS=linux go build -trimpath -o /tmp/arima-api ./cmd/erp-api
echo "Preflight passed. No deployment or migration was performed."
