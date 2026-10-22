# ERP Bridge Contracts

## Trust boundary
The AI orchestrator is an untrusted caller. The Go ERP core remains authoritative for identity, tenant scope, authorization, business rules, persistence, and audit. AI-generated arguments are data, not authority.

## Transports
- OpenAPI 3.1: HTTP contract for compatibility and debugging.
- Protobuf/gRPC: typed internal service contract when justified.
- Keep semantics aligned and add compatibility checks before publishing SDKs.

## Action lifecycle
1. Validate input and current resource version without mutation.
2. Propose action with a caller-scoped idempotency key.
3. Apply policy and determine whether independent human approval is required.
4. Verify approver permissions and separation-of-duties rules.
5. Execute only after authorization, policy, approval, and expected-version checks pass.
6. In one database transaction, persist the domain mutation, idempotency result, audit event, and outbox event.
7. Publish outbox events asynchronously with retries and consumer deduplication.

## Error semantics
- 400 malformed/invalid request; 401 invalid authentication; 403 unauthorized.
- 404 absent or tenant-invisible resource; 409 idempotency/version/state conflict.
- 429 rate limit; 5xx unexpected failure with correlation ID and no internal details.

## Implementation status
Only health routes are implemented in the initial executable. Resource/action endpoints in the API contracts are not live routes. Readiness must check real dependencies before production.
