# ARIMA CI integration secrets and credential rotation

## GitHub Actions → GitLab canonical validation

The GitHub workflow `.github/workflows/gitlab-validation.yml` calls the GitLab Pipeline API for project ID `85323260` and configuration ref `chore/github-source-ci`.

The latest observed failure is definitive: the GitHub Actions secret `GITLAB_API_TOKEN` is empty. The workflow fails closed before making a GitLab API request. This is a configuration blocker, not a Go or YAML test failure.

### Create the required secret

1. In GitLab, open the ARIMA project `nyeinpyaesone-ui-group/nyeinpyaesone-ui` (project ID `85323260`).
2. Open **Settings → Access Tokens**.
3. Create a dedicated project access token for CI, not a personal account token.
4. Use the minimum project role that can create and read pipelines (start with **Developer**); select the `api` scope because the workflow creates a pipeline and polls its status.
5. Set a short expiry and record the owner/rotation date in the team's approved credential inventory.
6. Copy the value once into **GitHub → nyeinpyaesone-ui/arima → Settings → Secrets and variables → Actions → New repository secret**.
7. Set the exact name `GITLAB_API_TOKEN`. Never add the value to repository files, GitHub variables, workflow logs, Notion, Linear or chat.
8. Re-run the GitHub workflow on a trusted same-repository branch and confirm that it creates a pipeline for the exact GitHub source SHA.
9. Confirm GitLab pipeline jobs actually execute and that the pipeline succeeds. Do not make the GitLab check required until this succeeds repeatedly.

The workflow intentionally skips fork-originated pull requests because repository secrets must not be exposed to untrusted fork code.

## Exposed GitLab runner credential

A GitLab project metadata response exposed a runner-registration credential field. Treat the value as compromised.

1. Open the GitLab project **Settings → CI/CD → Runners**.
2. Identify whether the exposed value is a legacy project registration token or a runner authentication token.
3. Rotate/revoke the exposed credential using GitLab's supported UI for that credential type. If a runner uses the old credential, register or re-register it with a fresh runner authentication token.
4. Verify the expected runner is online and assigned to the correct project/group.
5. Confirm new pipeline jobs are picked up by the intended runner.
6. Do not paste the replacement token into this repository or this document.

If GitLab requires account identity verification before allowing pipeline execution, complete the normal GitLab verification flow. Do not bypass the requirement or broaden token permissions to work around it.

## Secret inventory

| Name | Storage | Consumer | Status |
|---|---|---|---|
| `GITLAB_API_TOKEN` | GitHub Actions repository secret | GitHub workflow triggering/polling GitLab | Missing in latest observed run |
| GitLab runner authentication/registration credential | GitLab runner settings | GitLab runner registration/authentication | Treat as exposed; rotate |

No secret values belong in this runbook.