# Runtime configuration runbook

## Scope

This runbook covers HTTP listener/timeouts and optional OAuth token introspection. Database connectivity, persistence, tenant-isolation enforcement for stored records, and business mutations are not implemented.

## Supported HTTP variables

| Variable | Default | Validation |
| --- | --- | --- |
| `ARIMA_ENV` | development | Set to `production` to require authentication configuration |
| `ARIMA_ADDR` | `:8080` | Must be `host:port` or `:port`; port must be 1–65535 |
| `ARIMA_READ_HEADER_TIMEOUT` | `5s` | Go duration greater than 0 and no more than 5m |
| `ARIMA_READ_TIMEOUT` | `15s` | Go duration greater than 0 and no more than 5m |
| `ARIMA_WRITE_TIMEOUT` | `15s` | Go duration greater than 0 and no more than 5m |
| `ARIMA_IDLE_TIMEOUT` | `60s` | Go duration greater than 0 and no more than 5m |

## Optional authentication variables

Configure all five or none:

- `ARIMA_OIDC_INTROSPECTION_URL`: HTTPS token introspection endpoint (DNS hostname).
- `ARIMA_OIDC_ISSUER`: exact expected issuer claim.
- `ARIMA_OIDC_AUDIENCE`: required audience.
- `ARIMA_OIDC_CLIENT_ID`: introspection client ID.
- `ARIMA_OIDC_CLIENT_SECRET`: secret-manager value; never commit it.

The identity provider must validate token signatures and active/revocation status and return RFC 7662 JSON with `active`, `iss`, `aud`, `exp`, `sub`, and `tenant_id`. Partial configuration fails startup. When `ARIMA_ENV=production`, all five authentication values are mandatory; otherwise startup fails. Outside production, if all five are unset, the service starts for health probes but `GET /api/v1/me` returns 503.

See [environment example](../../configs/arima.env.example) and [authentication boundary](../security/authentication-boundary.md). The application does not automatically load dotenv files.

## Configure and run locally

From the repository root:

```sh
export ARIMA_ENV=development
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

In another terminal:

```sh
curl --fail --silent --show-error http://127.0.0.1:8080/health/live
curl --fail --silent --show-error http://127.0.0.1:8080/health/ready
curl -i http://127.0.0.1:8080/api/v1/me
```

The first two endpoints report process-level health only. Without identity configuration, the protected identity route should return 503. With configuration, missing/invalid bearer credentials should return 401 and a valid token should return the verified identity.

## Deployment rules

1. Set `ARIMA_ENV=production` in production. The executable refuses to start without complete authentication configuration.
2. Store the client secret in the deployment secret manager; do not commit real environment files or credentials.
3. Bind to `:8080` inside a container only when the platform/network policy expects all interfaces. For local-only development, use `127.0.0.1:8080`.
4. Put TLS termination at a trusted ingress/load balancer or add explicit TLS configuration before exposing the service directly to an untrusted network.
5. Keep the listener private until provider validation, authorization, dependency-aware readiness, tenant-scoped persistence, and operational controls are implemented.
6. Change timeouts only from observed request-duration and traffic data; defaults are not measured production SLOs.
7. Treat invalid configuration as a deployment failure. Do not silently fall back from malformed values.

## Verification and rollback

- Require green GitHub Actions for the exact proposed commit before merging.
- Test wrong issuer/audience, expired/revoked tokens, provider errors, missing tenant claims, and secret rotation against the real identity provider.
- Verify bind address, liveness response, logs, and restart behavior in the target environment.
- On a configuration rollout failure, restore the previous known-good environment values and redeploy the prior image. No database migrations are included in this stage.
