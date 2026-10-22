# Authentication boundary

`internal/auth` defines an authentication seam; it does not itself validate JWTs or establish trust in an identity provider. Protected handlers must be wrapped with `auth.RequireBearer` and supplied with a production `TokenVerifier` that validates signature, issuer, audience, expiry/not-before, and the approved key-rotation policy. Never decode a JWT and trust its claims without cryptographic verification.

## Fail-closed behavior

- A missing verifier returns `503 Service Unavailable`; protected operations must not run while authentication is unconfigured.
- Missing or malformed bearer credentials, verifier errors, or principals missing a subject or tenant ID return `401 Unauthorized`.
- Only a verified principal is placed in request context. Handlers should obtain it with `auth.PrincipalFromContext`.
- Authentication is not authorization. Every data access and mutation must still enforce tenant scoping and role/policy checks server-side.

## Current scope

This is a tested middleware boundary, not a complete identity-provider integration. No business mutation endpoints are enabled by this change. Before production use, implement and configure a vetted OIDC/JWT verifier, add issuer/audience/key-rotation configuration and integration tests, and add tenant-isolation tests at the persistence layer.
