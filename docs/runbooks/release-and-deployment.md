# Release and deployment runbook

## Scope

The release workflow produces Linux `amd64` and `arm64` API binaries only after the release candidate passes the same Go dependency-graph, formatting, vet, vulnerability, race, and PostgreSQL integration checks. A version tag (`v*`) publishes binaries, SHA-256 checksums, and GitHub build-provenance attestations.

**Publishing a GitHub release is not deploying the service.** No production environment, host, cluster, registry, or deployment credentials are assumed or configured by this repository. Do not add automatic production deployment until an owner-approved target, environment protection, credentials model, health verification, and rollback procedure exist.

## Release procedure

1. Review and merge the approved change through repository branch protection and required checks.
2. Create and push a version tag only from the reviewed release commit, for example `v1.0.0`. Never tag an unreviewed feature branch.
3. Monitor the GitHub Actions **Release** workflow. It must pass verification before building both Linux architectures.
4. Confirm the GitHub release contains both binaries, `SHA256SUMS`, and provenance attestations. Verify checksums before installation.
5. Record the release tag, source commit SHA, workflow run URL, checksum, deployment operator, target, and deployment time in the change record.

## Deployment gate (required before production use)

- Identify the production target and service owner; define the supported architecture and operating system.
- Provision secrets through the target platform's secret manager. Do not commit credentials or bake them into artifacts.
- Configure a dedicated non-root service identity, least-privilege filesystem access, explicit environment configuration, TLS termination, network policy, resource limits, and restart policy.
- Use a separate least-privilege runtime database role; execute reviewed migrations as a distinct migration identity.
- Deploy to staging first. Verify startup, OIDC discovery and issuer/audience settings, database connectivity, forced tenant RLS, readiness/liveness behavior, and representative authorization-denial cases.
- Promote the exact same immutable artifact to production after approval. Do not rebuild between staging and production.
- Run post-deployment smoke checks and observe error rate, latency, restarts, database pool saturation, and outbox lag before declaring success.

## Rollback

1. Stop promotion and preserve the failing release's logs, commit SHA, configuration version, and migration state.
2. Re-deploy the last known-good immutable binary and restore its approved configuration.
3. Do not automatically run down-migrations in production. Database rollback must be reviewed for data-loss and forward/backward compatibility; prefer a tested forward-fix when destructive rollback is unsafe.
4. Verify readiness, authenticated and denied requests, tenant isolation, and key operational metrics.
5. Document the incident and add a regression test before retrying the release.

## Automation boundaries

The workflow intentionally does not deploy to an unspecified host or cluster. Add a separate deployment workflow only after the target is explicit, using a protected GitHub Environment, narrowly scoped permissions, OIDC federation instead of long-lived cloud keys where supported, concurrency protection, deployment approval, post-deploy health checks, and a tested rollback mechanism.
