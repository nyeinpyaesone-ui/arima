# Test strategy

As the service grows, require:
- Unit tests for domain rules and state transitions.
- HTTP/gRPC contract tests against OpenAPI and Protobuf.
- PostgreSQL integration tests for transactions, idempotency, audit, outbox, and concurrency.
- Tenant-isolation and authorization negative tests.
- End-to-end tests for propose → approve → execute.
- Load and failure-mode tests before performance claims or service extraction.
