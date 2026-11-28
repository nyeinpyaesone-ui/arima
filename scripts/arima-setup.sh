#!/usr/bin/env bash
set -Eeuo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
fail() { echo "ERROR: $*" >&2; exit 1; }
need() { command -v "$1" >/dev/null 2>&1 || fail "Missing command: $1"; }
compose() { docker compose --env-file "$ROOT/.env" -f "$ROOT/compose.yaml" "$@"; }
worker_compose() { docker compose --env-file "$ROOT/.env" -f "$ROOT/compose.yaml" -f "$ROOT/compose.worker.yaml" "$@"; }
env_file() {
  if [ ! -f .env ]; then cp configs/arima.env.example .env; chmod 600 .env; fi
  git check-ignore -q .env || fail ".env must be Git-ignored."
}
preflight() {
  for c in git go node npm docker curl jq; do need "$c"; done
  [ "$(go env GOVERSION)" = "go1.26.11" ] || fail "Use Go 1.26.11 to match go.mod and CI."
  [ "$(node -p 'Number(process.versions.node.split(".")[0])')" -ge 22 ] || fail "Node.js 22+ required."
  docker compose version >/dev/null || fail "Docker Compose v2 required."
  echo "Toolchain verified: $(go version), $(node --version), $(docker compose version --short)"
}
wait_health() {
  url="$1"; state="$2"; label="$3"; n=0
  while [ "$n" -lt 45 ]; do
    body="$(curl -fsS --connect-timeout 2 --max-time 5 "$url" 2>/dev/null || true)"
    if printf '%s' "$body" | jq -e --arg state "$state" '.status == $state' >/dev/null 2>&1; then
      echo "$label: $state"; return
    fi
    n=$((n + 1)); sleep 2
  done
  fail "$label failed at $url"
}
verify() {
  preflight; env_file
  bash -n scripts/env-preflight.sh
  bash -n scripts/arima-setup.sh
  sh -n scripts/migrate-local.sh
  compose config --quiet
  go mod download; go mod verify
  [ -z "$(gofmt -l ./cmd ./internal)" ] || fail "Run gofmt on Go source files."
  go vet ./...; go test -race ./...
  tmp="$(mktemp -d)"
  CGO_ENABLED=0 go build -trimpath -o "$tmp/erp-api" ./cmd/erp-api
  CGO_ENABLED=0 go build -trimpath -o "$tmp/outbox-worker" ./cmd/outbox-worker
  rm -rf "$tmp"
  (cd web && npm ci --no-audit --no-fund && npm run typecheck && npm run build)
  echo "Verification passed; PostgreSQL integration tests remain authoritative in GitHub CI."
}
up() {
  preflight; env_file
  docker info >/dev/null 2>&1 || fail "Docker Engine is not running."
  compose config --quiet; compose up --build -d api
  cfg="$(compose config --format json)"
  port="$(printf '%s' "$cfg" | jq -r '.services.api.ports[]? | select(.target == 8080) | .published // empty' | head -n 1)"
  [ -n "$port" ] || port=8080
  base="http://127.0.0.1:$port"
  wait_health "$base/health/live" live "API liveness"
  wait_health "$base/health/ready" ready "API readiness"
  migrations="$(compose exec -T database sh -c 'psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT count(*) FROM arima_schema_migrations"')"
  [ "$migrations" -ge 3 ] || fail "Expected migrations 0001-0003; found $migrations."
  roles="$(compose exec -T database sh -c 'psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT count(*) = 2 AND bool_and(NOT rolsuper AND NOT rolbypassrls) FROM pg_roles WHERE rolname IN ('\''arima_api'\'','\''arima_outbox'\'')"')"
  [ "$roles" = t ] || fail "Runtime roles are missing or overprivileged."
  worker_access="$(compose exec -T database sh -c 'psql -X -U "$POSTGRES_USER" -d "$POSTGRES_DB" -Atqc "SELECT has_table_privilege('\''arima_outbox'\'','\''tenant_records'\'','\''SELECT'\'')"')"
  [ "$worker_access" = f ] || fail "Worker can read tenant business records."
  echo "API: $base; migrations: $migrations; runtime role checks: passed."
}
worker() {
  preflight; env_file
  docker info >/dev/null 2>&1 || fail "Docker Engine is not running."
  cfg="$(worker_compose config --format json)"
  url="$(printf '%s' "$cfg" | jq -r '.services["outbox-worker"].environment.ARIMA_OUTBOX_WEBHOOK_URL // empty')"
  token="$(printf '%s' "$cfg" | jq -r '.services["outbox-worker"].environment.ARIMA_OUTBOX_WEBHOOK_TOKEN // empty')"
  case "$url" in https://*) ;; *) fail "Set a real HTTPS ARIMA_OUTBOX_WEBHOOK_URL in .env." ;; esac
  case "$url" in *example.invalid*) fail "Placeholder webhook URL is not allowed." ;; esac
  [ -n "$token" ] && [ "$token" != replace-before-running-worker ] || fail "Set ARIMA_OUTBOX_WEBHOOK_TOKEN in .env."
  worker_compose up --build -d api outbox-worker
  port="$(printf '%s' "$cfg" | jq -r '.services["outbox-worker"].ports[]? | select(.target == 8081) | .published // empty' | head -n 1)"
  [ -n "$port" ] || port=8081
  wait_health "http://127.0.0.1:$port/health/ready" ready "Worker database readiness"
  echo "Worker readiness checks database connectivity, not webhook delivery."
}
status() {
  env_file; compose ps --all
  cfg="$(compose config --format json)"
  port="$(printf '%s' "$cfg" | jq -r '.services.api.ports[]? | select(.target == 8080) | .published // empty' | head -n 1)"
  [ -n "$port" ] || port=8080
  curl -fsS "http://127.0.0.1:$port/health/ready" || true
}
down() { env_file; worker_compose down --remove-orphans; }
usage() {
  echo "Commands: preflight | verify | up | worker | status | down | all"
  echo "all = verify then up. down preserves the PostgreSQL volume."
  echo "worker requires a real HTTPS receiver; no remote deploys are performed."
}
[ -f go.mod ] || fail "Run this script from the repository root."
if [ "$#" -gt 0 ]; then
  cmd="$1"
else
  cmd=help
fi
case "$cmd" in
  preflight) preflight ;;
  verify) verify ;;
  up) up ;;
  worker) worker ;;
  status) status ;;
  down) down ;;
  all) verify; up ;;
  ""|help|-h|--help) usage ;;
  *) usage; fail "Unknown command: $cmd" ;;
esac
