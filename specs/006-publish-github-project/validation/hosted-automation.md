# Hosted Automation Validation

**Date:** 2026-09-08
**Repository:** `github.com/pluque01/orza`
**Visibility:** private

## CI Event And Failure Matrix

- `scripts/ci_workflow_test.sh` verified push, pull-request, merge-queue, and manual triggers; full-SHA
  action pins; read-only permissions; uncensored build coverage; and the stable `required` aggregate name.
- `scripts/ci_gate_test.sh` exercised every validation category as success, failure, cancellation, and
  skipped input. Only the all-success matrix passed, so the aggregate gate fails closed.
- Pull request #1 passed the protected `required` check and merged without a ruleset bypass.
- Manual CI run `34259177809` passed at revision
  `118647a66d76b96208868fa7d8a8fcd094cc8170`.
- Pull request #2 passed all four release builds, quality, vendor, tree/history secret scans, and the
  protected `required` check in run `34261932371`. Two secret-job attempts failed before scanning when
  `proxy.golang.org` returned HTTP/2 stream errors while Nix populated a cold store; the aggregate gate
  failed as designed. A clean retry passed both scans and the aggregate gate, after which the PR merged
  without bypass as revision `09c2783`.

## Renovate Validation

- Static policy, workflow, and four-week simulation contracts passed in
  `scripts/renovate_config_test.sh`, `scripts/dependencies_workflow_test.sh`, and
  `scripts/dependencies_cycle_test.sh`.
- Initial hosted dry-run `34259177805` exposed invalid repository configuration while Renovate still
  returned success. The invalid Go option and Actions rule combination were removed, and the vulndb
  fallback was changed to the supported `git-refs` datasource with an explicit `master` value and SHA
  digest capture.
- Both manual and scheduled commands now set `--config-validation-error=true`; regression contracts
  require the flag in both paths. Invalid configuration therefore exits non-zero instead of producing a
  false-green workflow.
- Corrected manual run `34264753236` passed on `main`. Renovate 43.150.1 started, extracted 72
  dependencies from Go modules, GitHub Actions, and the vulndb regex fallback, and finished without a
  configuration or missing-datasource error. The scheduled mutating job was skipped for the manual event.
- No onboarding branch, dependency branch, pull request, weekly marker, tag, visibility change, or
  release was created by either manual dry-run.

## Four-Week And Proposal Bounds

- The deterministic simulation acquired one `automation/renovate/2026-Www` marker in each of four
  synthetic weeks and made every same-week rerun a no-op.
- A simulated failed Renovate cycle retained its marker and its rerun remained a no-op, preventing a
  second mutating cycle in the same UTC week.
- The first five synthetic proposals were admitted and the sixth was blocked, matching both branch and
  pull-request limits of five.
- Proposal assertions covered old/new versions, affected Go manifest/vendor files, ordinary CI,
  disabled automerge, and actionable failure diagnostics.
- Compatible updates remain grouped within their own ecosystem; major updates remain isolated behind
  dashboard approval. Nixpkgs and the vulnerability database remain separate.

T053 completed without executing a mutating Renovate cycle against the repository.
