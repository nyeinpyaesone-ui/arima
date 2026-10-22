# Database migrations

Place versioned, reviewed SQL migrations here. Do not mutate schema at application startup. Select and test migration tooling before adding persistent business functionality.

- Prefer additive, backward-compatible expand/migrate/contract changes.
- Run migrations as a controlled deployment step using a least-privilege migration identity.
- Verify backup and restore procedures before high-risk schema changes.
- Never store credentials in migration files.
