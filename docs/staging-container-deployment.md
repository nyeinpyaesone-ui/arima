# Staging container deployment

The canonical, maintained staging deployment instructions are in [`docs/runbooks/staging-deployment.md`](runbooks/staging-deployment.md).

That runbook documents the current guarded workflow (`.github/workflows/deploy-staging.yml`), required GitHub Environment configuration, provider-adapter contract, API and private worker readiness, runtime secret boundaries, migration responsibilities, rollback requirements, and remaining production gates.

Use the canonical runbook as the source of truth. This page is retained as a compatibility pointer for existing links; it intentionally does not duplicate deployment configuration or claim that a provider adapter or live staging rollout has been completed.
