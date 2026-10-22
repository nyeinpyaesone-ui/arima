# Contributing

## Workflow
1. Create a focused branch from `main`.
2. Keep changes small; document any compatibility break.
3. Add tests for behavior and regression cases.
4. Run `make check` before opening a pull request.
5. Document security, migration, and operational impact.

## Design rules
- Keep business policy in application/domain layers, not HTTP handlers or AI prompts.
- Derive tenant identity from verified authentication claims; never trust request payloads for tenant scope.
- Mutations require authorization, policy validation, idempotency, audit records, and transactional outbox writes.
- Use optimistic concurrency for mutable resources.
- Never log access tokens, secrets, or unnecessary personal/customer data.
- Avoid dependencies and infrastructure without a measurable reason.

## API changes
Update OpenAPI and Protobuf contracts together where applicable. Explain compatibility impact and add contract checks before publishing generated SDKs.
