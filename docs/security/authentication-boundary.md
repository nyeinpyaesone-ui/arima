# Authentication boundary

## Implementation

The executable uses the maintained `github.com/coreos/go-oidc/v3/oidc` verifier. It discovers provider metadata from the configured issuer and validates JWT signature, exact issuer, configured audience, expiry and not-before using the provider's JWKS endpoint. The remote key set supports signing-key rotation. Provider discovery is bounded by a 10-second startup context; production startup fails if discovery or authentication configuration fails.

Required variables:

- `ARIMA_OIDC_ISSUER`: exact issuer URL from the provider's discovery metadata.
- `ARIMA_OIDC_AUDIENCE`: API audience expected in the access token.

Configure both or leave both unset outside production. The verified token must contain non-empty `sub` and `tenant_id` claims. Use an access token intended for ARIMA; do not assume an ID token issued to a different client is an acceptable API credential.

## Protected routes

`GET /api/v1/me` is wired through bearer middleware. Valid credentials return only the verified subject and tenant ID. Customer create, read and list routes are also wired through bearer middleware and enforce tenant-scoped permissions inside the tenant transaction. Health endpoints remain unauthenticated for platform probes. When authentication is absent outside production, protected routes fail closed with 503; missing or invalid credentials return 401.

## Security boundaries and limitations

- Signature, issuer, audience, expiry/not-before validation and JWKS refresh are delegated to the vetted OIDC library; the application does not parse JWTs manually.
- The selected identity provider and its access-token claim contract have not yet been integration-tested. Before production, test invalid signatures, wrong issuer/audience, expiry, not-before, unknown/rotated signing keys, missing tenant claim, provider outage and revocation semantics.
- A tenant ID claim is an identity attribute, not sufficient authorization. Migration 0003 adds tenant-scoped roles, permissions and membership status under forced RLS. `PostgresAuthorizer` evaluates active membership grants; mutation and list operations additionally call `TenantTx.RequirePermission` inside the same transaction as the data operation. A transaction that has already passed its permission check may finish after a concurrent revocation commits; the revocation affects permission checks that run afterward. This is not immediate cancellation of in-flight operations.
- The customer API currently supports tenant-authorized create, read and bounded keyset list operations. Other ERP resource/action routes remain planned contracts; the PostgreSQL tenant repository and authorization schema are foundational primitives, not a complete ERP application.
- Keep the listener private until identity-provider tests, database grants/RLS tests, authorization, TLS ingress, and operational controls are reviewed.

## Deployment verification

1. Set issuer and audience from authoritative provider configuration; never guess either value.
2. Configure `ARIMA_DATABASE_URL` from a secret manager. Production startup requires a successful PostgreSQL ping.
3. Verify no authentication config gives 503 from `GET /api/v1/me`, invalid credentials give 401, and a valid access token returns the expected subject and tenant.
4. Exercise key rotation and provider outage in staging before exposure.
5. Keep the runtime database role non-superuser and without `BYPASSRLS`; test tenant isolation under that role.
6. Require green GitHub Actions for the exact head commit before merging.
