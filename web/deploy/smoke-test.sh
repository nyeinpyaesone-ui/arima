#!/usr/bin/env bash
set -euo pipefail

: "${ARIMA_API_BINARY:?Set ARIMA_API_BINARY to the built Go API executable}"
: "${WEB_DIST_DIR:?Set WEB_DIST_DIR to the built Vite dist directory}"

for command in nginx envsubst curl; do
  command -v "$command" >/dev/null 2>&1 || {
    echo "Required command not found: $command" >&2
    exit 1
  }
done

tmp_dir="$(mktemp -d)"
api_pid=""
cleanup() {
  if [[ -f "$tmp_dir/nginx.pid" ]]; then
    nginx -s quit -c "$tmp_dir/nginx.conf" -p "$tmp_dir/" >/dev/null 2>&1 || true
  fi
  if [[ -n "$api_pid" ]]; then
    kill "$api_pid" >/dev/null 2>&1 || true
    wait "$api_pid" 2>/dev/null || true
  fi
  rm -rf "$tmp_dir"
}
trap cleanup EXIT

web_root="$tmp_dir/html"
mkdir -p "$web_root"
cp -R "$WEB_DIST_DIR"/. "$web_root"/

export ARIMA_API_UPSTREAM="http://127.0.0.1:18081"
export ARIMA_WEB_ROOT="$web_root"
envsubst '${ARIMA_API_UPSTREAM} ${ARIMA_WEB_ROOT}' \
  < web/deploy/nginx.conf.template \
  > "$tmp_dir/default.conf"

cat > "$tmp_dir/nginx.conf" <<NGINX
pid $tmp_dir/nginx.pid;
error_log stderr;
events {}
http {
    include /etc/nginx/mime.types;
    default_type application/octet-stream;
    access_log off;
    include $tmp_dir/default.conf;
}
NGINX

nginx -t -c "$tmp_dir/nginx.conf" -p "$tmp_dir/"
ARIMA_ADDR="127.0.0.1:18081" "$ARIMA_API_BINARY" &
api_pid="$!"

for _ in $(seq 1 30); do
  if curl --fail --silent http://127.0.0.1:18081/health/live >/dev/null; then
    break
  fi
  sleep 0.5
done
curl --fail --silent http://127.0.0.1:18081/health/live >/dev/null

nginx -c "$tmp_dir/nginx.conf" -p "$tmp_dir/"

for _ in $(seq 1 30); do
  if curl --fail --silent http://127.0.0.1:8080/ >/dev/null; then
    break
  fi
  sleep 0.5
done

curl --fail --silent http://127.0.0.1:8080/ > "$tmp_dir/index.html"
grep -q "ARIMA — Precision Operations" "$tmp_dir/index.html"
curl --fail --silent --head http://127.0.0.1:8080/ \
  | grep -qi '^Content-Type: text/html'
curl --fail --silent http://127.0.0.1:8080/overview \
  | grep -q "ARIMA — Precision Operations"

curl --fail --silent http://127.0.0.1:8080/health/live \
  | grep -q '"status":"live"'
curl --fail --silent http://127.0.0.1:8080/health/ready \
  | grep -q '"status":"ready"'

unknown_status="$(curl --silent --output /dev/null --write-out '%{http_code}' \
  http://127.0.0.1:8080/health/not-implemented)"
test "$unknown_status" = "404"

curl --fail --silent --head http://127.0.0.1:8080/ \
  | grep -qi '^Content-Security-Policy:'

echo "Web/API integration smoke test passed: SPA, same-origin health proxy, 404 boundary, and security headers."
