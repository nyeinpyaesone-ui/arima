# Runtime configuration runbook

## Scope

This runbook defines the executable's HTTP listener and timeout configuration. It does not configure an identity provider, database, tenant, or business integration. Those are not wired into the current executable yet.

## Supported variables

| Variable | Default | Validation |
| --- | --- | --- |
| `ARIMA_ADDR` | `:8080` | Must be `host:port` or `:port`; port must be 1–65535 |
| `ARIMA_READ_HEADER_TIMEOUT` | `5s` | Go duration greater than 0 and no more than 5m |
| `ARIMA_READ_TIMEOUT` | `15s` | Go duration greater than 0 and no more than 5m |
| `ARIMA_WRITE_TIMEOUT` | `15s` | Go duration greater than 0 and no more than 5m |
| `ARIMA_IDLE_TIMEOUT` | `60s` | Go duration greater than 0 and no more than 5m |

See [arima.env.example](../../configs/arima.env.example). The example file is documentation only; the application does not automatically load a dotenv file. Set variables in the process environment or deployment platform.

## Configure and run locally

From the repository root:

```sh
export ARIMA_ADDR='127.0.0.1:8080'
export ARIMA_READ_HEADER_TIMEOUT='5s'
export ARIMA_READ_TIMEOUT='15s'
export ARIMA_WRITE_TIMEOUT='15s'
export ARIMA_IDLE_TIMEOUT='60s'

go test ./...
go vet ./...
test -z "$(gofmt -l ./cmd ./internal)"
go run ./cmd/erp-api
```

In a second terminal, verify the process:

```sh
curl --fail --silent --show-error http://127.0.0.1:8080/health/live
curl --fail --silent --show-error http://127.0.0.1:8080/health/ready
```

Both endpoints currently report process-level health. Readiness does not yet check external dependencies because database and identity-provider integrations are not wired.

## Deployment rules

1. Set configuration explicitly in the deployment platform; do not commit real environment files or credentials.
2. Bind to `:8080` inside a container only when the platform/network policy expects all interfaces. For local-only development, use `127.0.0.1:8080`.
3. Put TLS termination at a trusted ingress/load balancer or add explicit TLS configuration before exposing the service directly to an untrusted network.
4. Keep the listener private until authentication, authorization, dependency-aware readiness, and operational controls are implemented.
5. Change timeouts only from observed request-duration and traffic data; the current defaults are starting values, not measured production SLOs.
6. Treat invalid configuration as a deployment failure. Do not silently fall back from malformed values.

## Verification and rollback

- Run the full CI workflow and require a green result before merging.
- In the target environment, verify the configured bind address, liveness response, logs, and restart behavior.
- If a configuration rollout fails, restore the previous known-good environment values and redeploy the prior image. Do not alter database schemas as part of this rollback; this change introduces no database migrations.
