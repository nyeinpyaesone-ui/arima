# Security Policy

Do not publish exploitable vulnerabilities in public issues. Contact the repository owner privately through GitHub's security reporting capability where available.

## Required before production
- Verified JWT issuer, audience, expiry, key rotation, and algorithm allow-list.
- Tenant-scoped authorization on every resource and action.
- Independent approver identity where separation-of-duties policy requires it.
- Idempotency keys and optimistic concurrency on writes.
- Audit trail and transactional outbox in the same transaction as business mutations.
- External secret management and least-privilege runtime identity.
- Dependency, image, and SBOM scanning in release pipeline.
- Rate limits, request size limits, timeouts, and redacted structured logs.

The current scaffold does not implement authentication, database access, action execution, or dependency-aware readiness. Do not expose it as a production ERP API until these controls are implemented and tested.
