# Protect the canonical ARIMA main branch

## Observed configuration

Ruleset ID: `27686739` (name: `default`, active).

The live ruleset currently targets `refs/heads/default`, not `refs/heads/main`. Its only rule is a required-deployments rule referencing two stale environment names:

- `chore/production-config-baseline - arima PR #1`
- `development`

This ruleset therefore does not provide the intended PR and CI protection for `main`.

## Required check contexts

Verified on PR #23 head `f6b368c0f36c236f479b772377419c06c8bc6a22`:

| Check context | GitHub Actions workflow |
|---|---|
| `verify` | CI |
| `verify-web` | Web CI |
| `Build container without publishing` | Container Image |

Web CI is configured in this PR to run for every push/PR to main, rather than using path filters. That is required because a path-filtered workflow can leave a required check in a skipped/pending state when a PR does not touch `web/**`.

Do not require the GitLab validation context until GitLab account verification is complete and the canonical-SHA pipeline passes. Do not require deployment environments as a substitute for CI checks.

## Apply through a GitHub administrator session

The connected GitHub integration supports reading rulesets but not writing them. Apply this configuration with an administrator-authenticated GitHub CLI session. Do not use a token pasted into chat; use `gh auth login` with the account's normal secure authentication.

First re-read the live ruleset and check contexts:

```bash
gh api repos/nyeinpyaesone-ui/arima/rulesets/27686739
gh api repos/nyeinpyaesone-ui/arima/commits/f6b368c0f36c236f479b772377419c06c8bc6a22/check-runs \
  --jq '.check_runs[] | [.name, .conclusion, (.app.id|tostring)] | @tsv'
```

Create `/tmp/arima-main-ruleset.json` with this reviewed payload:

```json
{
  "name": "main-protection",
  "target": "branch",
  "enforcement": "active",
  "conditions": {
    "ref_name": {
      "include": ["refs/heads/main"],
      "exclude": []
    }
  },
  "rules": [
    {
      "type": "pull_request",
      "parameters": {
        "required_approving_review_count": 1,
        "dismiss_stale_reviews_on_push": true,
        "require_code_owner_review": false,
        "require_last_push_approval": true,
        "required_review_thread_resolution": true,
        "allowed_merge_methods": ["merge", "squash", "rebase"]
      }
    },
    {
      "type": "required_status_checks",
      "parameters": {
        "strict_required_status_checks_policy": true,
        "do_not_enforce_on_create": false,
        "required_status_checks": [
          {"context": "verify", "integration_id": 15368},
          {"context": "verify-web", "integration_id": 15368},
          {"context": "Build container without publishing", "integration_id": 15368}
        ]
      }
    },
    {"type": "deletion"},
    {"type": "non_fast_forward"}
  ],
  "bypass_actors": []
}
```

The payload intentionally removes all bypass actors and stale deployment rules. Review this consequence before applying. If an emergency bypass is required, define a named, documented, least-privilege bypass actor and a non-`always` mode; do not silently restore broad bypass.

Apply and read back:

```bash
gh api --method PUT repos/nyeinpyaesone-ui/arima/rulesets/27686739 \
  --input /tmp/arima-main-ruleset.json
gh api repos/nyeinpyaesone-ui/arima/rulesets/27686739
```

Verify the returned conditions target only `refs/heads/main`, the three exact check contexts are present, the pull-request rule requires one approval, stale reviews are dismissed, review threads must be resolved, force pushes and branch deletion are blocked, and the stale deployment requirements are gone.

## Acceptance test

1. Open a disposable test PR against `main`.
2. Confirm it cannot merge without one independent approval.
3. Confirm it cannot merge while any of the three required checks is missing or failing.
4. Push another commit and confirm the previous approval is dismissed.
5. Confirm unresolved review threads block merge.
6. Confirm GitHub rejects direct force-push and branch deletion attempts.
7. Re-read the ruleset and save the returned JSON as change evidence.

Do not merge application PRs solely because the ruleset is corrected. Each PR still needs current-head green checks and independent review.
