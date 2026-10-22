# ARIMA staging deployment runbook

## Scope and maturity

This deployment path is **staging only** and deploys the current main-branch Go HTTP scaffold. The current service implements root and process-level health endpoints; it does not yet implement the business API, database-backed readiness, production OIDC enforcement, or ERP workflows. A successful deployment proves packaging and transport, not production readiness.

## Pipeline

1. CI checks Go formatting, vet, tests and build.
2. Container Image builds on pull requests without publishing. A push to `main` publishes an immutable commit-tagged image to GHCR.
3. Deploy ARIMA Staging is manually dispatched from `main`, requires an explicit acknowledgement and the protected `staging` GitHub Environment.
4. The job validates required secrets, uses pinned host keys (no `ssh-keyscan` trust-on-first-use), authenticates to GHCR, transfers Compose configuration, deploys the commit-specific image, and checks `GET /health/ready`.
5. The service binds only to host loopback on port 8080. An administrator must configure a separately managed TLS reverse proxy and DNS before public access.

## Required GitHub Environment

Create a GitHub Environment named `staging`. Add the following environment-scoped secrets only after provisioning a dedicated staging host:

- `STAGING_HOST`: DNS name or IP of the approved staging server.
- `STAGING_USER`: dedicated, non-root deployment account with access to Docker/Compose.
- `STAGING_SSH_PRIVATE_KEY`: private key for that deployment account.
- `STAGING_KNOWN_HOSTS`: host key line obtained and verified out-of-band from the server administrator.
- `GHCR_USERNAME`: account allowed to read the package.
- `GHCR_READ_TOKEN`: narrowly scoped read-only package token.

Require a reviewer for the `staging` Environment if review is part of the repository's operating model. Restrict SSH access to the GitHub runner network only where practical; hosted runner egress IPs can change, so use a controlled self-hosted runner or a secure overlay if stable allowlisting is required. Never commit secret values.

## Host prerequisites

- Supported Linux host with Docker Engine and Compose v2 installed and patched.
- Dedicated deployment account; no SSH root login.
- Registry egress to `ghcr.io`.
- Port 8080 reserved for loopback-only application traffic.
- Independent reverse proxy with TLS and correct forwarded-header policy before external access.
- Monitoring, host patching, backup policy for any future persistent data, and documented operator access.

The current app is stateless and does not require PostgreSQL. Do not attach an unused database merely to make deployment look production-grade. Add PostgreSQL only with an implemented persistence layer, versioned migrations, least-privilege runtime and migration roles, and tested readiness checks.

## Rollback

1. Identify the last known-good image tag `ghcr.io/nyeinpyaesone-ui/arima:sha-<commit>`.
2. On the staging host, set `ARIMA_IMAGE` to that immutable tag and run `docker compose -f "$HOME/arima-staging/compose.yml" pull` followed by `docker compose -f "$HOME/arima-staging/compose.yml" up -d`.
3. Verify `curl --fail http://127.0.0.1:8080/health/ready`.
4. Record the failed commit, timestamps and redacted logs. Do not delete volumes or perform destructive database actions.

## Production blockers

- No production deployment job is provided.
- No staging host, DNS/TLS endpoint, GHCR package visibility, environment secrets or branch protection was configured by this change.
- Health readiness currently checks process response only, not dependencies.
- Business routes, tenant-aware persistence, auth policy, migration runner, observability, SLOs, backup/restore, security scanning and a verified release/provenance chain remain separate gates.
- Before any production deployment, implement and verify those capabilities, run end-to-end and recovery tests in staging, then create a separately reviewed production workflow with protected approvals and least-privilege identity.
