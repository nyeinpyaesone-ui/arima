# Web deployment boundary

This frontend is a static SPA. The included Nginx template serves the built Vite output and forwards the two implemented API health routes on the same origin.

## Required deployment inputs

- Build the frontend with Node.js 22+ from `web/`.
- Publish the contents of `web/dist/` as the Nginx document root (`/usr/share/nginx/html` in the example).
- Set `ARIMA_API_UPSTREAM` to the private origin of the Go API, for example `http://arima-api:8080`. It must be reachable from the Nginx runtime and must not include a path suffix.
- Set `ARIMA_WEB_ROOT` to the absolute filesystem path containing the built static files, for example `/usr/share/nginx/html`.
- Keep the API on a private service network. Expose the web entry point through the platform ingress and terminate TLS there.

## Render and validate the Nginx configuration

The official Nginx container images include `envsubst` through the `gettext` package in their standard entrypoint environment. Render only the upstream placeholder so Nginx variables such as `$host` and `$remote_addr` remain intact:

```sh
: "${ARIMA_API_UPSTREAM:?Set the private Go API origin}"
case "$ARIMA_API_UPSTREAM" in
  http://*|https://*) ;;
  *) echo "ARIMA_API_UPSTREAM must be an http(s) origin" >&2; exit 1 ;;
esac
: "${ARIMA_WEB_ROOT:?Set the absolute static document root}"
envsubst '${ARIMA_API_UPSTREAM} ${ARIMA_WEB_ROOT}' \
  < web/deploy/nginx.conf.template \
  > /etc/nginx/conf.d/default.conf
nginx -t
```

Copy the built `web/dist/` directory into the configured Nginx document root before starting Nginx. The smoke test at `web/deploy/smoke-test.sh` exercises the built app through Nginx and the Go API on one origin.

## Route contract

| Public path | Behavior |
|---|---|
| `/` and SPA routes | Serve the static app; unknown paths fall back to `index.html` |
| `/health/live` | Proxy to Go API liveness handler |
| `/health/ready` | Proxy to Go API readiness handler |
| Other `/health/*` paths | Return 404 |

The API handlers currently return HTTP 200 with JSON status values. Readiness is a scaffold response and does not yet validate database, broker, or other downstream dependencies. Do not interpret it as a production dependency-readiness guarantee.

## Platform-specific work still required

This repository does not identify a confirmed production hosting provider, ingress, service DNS name, domain, TLS certificate, or release promotion policy for the frontend. Configure those in the target platform rather than hardcoding assumptions here. Add a platform-specific deployment workflow only after those values and the staging environment are confirmed.
