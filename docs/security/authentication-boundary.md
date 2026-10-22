# Authentication boundary

## Implementation

The executable can use OAuth 2.0 token introspection (RFC 7662) through `internal/auth.IntrospectionVerifier`. The authorization server must authenticate the client, validate token signature and active status, and return the expected claims. ARIMA sends introspection requests over HTTPS with HTTP Basic client authentication and does not log bearer tokens.

Required environment variables (configure all or none):

- `ARIMA_OIDC_INTROSPECTION_URL`: absolute HTTPS endpoint, DNS hostname required.
- `ARIMA_OIDC_ISSUER`: exact expected `iss` claim.
- `ARIMA_OIDC_AUDIENCE`: required `aud` value.
- `ARIMA_OIDC_CLIENT_ID`: introspection client identifier.
- `ARIMA_OIDC_CLIENT_SECRET`: secret supplied by the deployment secret manager.

The verifier requires an active response, exact issuer, matching audience, future expiry, valid optional not-before, non-empty subject, and non-empty `tenant_id`. Introspection calls have a five-second timeout. If no auth variables are configured, protected routes fail closed with 503. Partial or malformed configuration stops startup.

## Protected route

`GET /api/v1/me` is wired through the bearer middleware. With valid authentication it returns only the verified subject and tenant ID. Health endpoints remain unauthenticated for platform probes. The root endpoint is informational only.

## Security boundaries and limitations

- The authorization server, not this API, validates the token's cryptographic signature and active status. Use a vetted provider that enforces approved signing algorithms, issuer, audience, expiry/not-before, revocation and key rotation.
- This is token introspection, not local JWT signature verification. Do not configure an endpoint that merely decodes JWT claims.
- A tenant ID claim is an identity attribute, not sufficient authorization by itself. Data access must still apply tenant predicates and role/policy checks server-side.
- The current API has no database persistence or business mutation routes. No tenant isolation guarantee for persisted records is claimed.
- Keep the listener private until deployment TLS, ingress restrictions, identity-provider configuration, authorization, dependency-aware readiness, and operational controls are reviewed.

## Deployment verification

1. Provision a dedicated introspection client and least-privilege secret in the identity provider.
2. Store the client secret in the deployment secret manager; never commit it or place it in logs.
3. Set all five environment variables, ensuring issuer and audience exactly match provider output.
4. Verify no credentials returns 503 from `GET /api/v1/me`, missing/invalid credentials return 401, and a valid token returns the expected subject and tenant.
5. Test expired, revoked, wrong-issuer, wrong-audience, and missing-tenant tokens against the actual identity provider before exposing the API.
6. Rotate the introspection secret through the secret manager and confirm the provider accepts the new credential before revoking the old one.
