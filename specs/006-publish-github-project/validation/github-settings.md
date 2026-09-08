# GitHub Settings Validation

**Date:** 2026-09-07
**Repository:** `github.com/pluque01/orza`
**Initial validation visibility:** private
**Current visibility:** public as of 2026-09-08

## Private Push

- SSH push created `main` and set `origin/main` as its upstream.
- Local and remote revisions matched at `815ca9b34dbe1260469c31a398b6eb894b49b7b4` after the
  initial push.
- The pipeline fix was subsequently pushed as `03fab64b7cd2574f796406a117a8431fc999080b`.
- An unauthenticated API request returned HTTP 404 while the authenticated repository API reported
  `visibility: private`.
- No credential value appeared in Git, workflow, configuration, or verification output.

## Metadata And Automation

- Repository name: `orza`
- Default branch: `main`
- Description: `A terminal-native SSH client with a TUI`
- Topics: `cli`, `go`, `ssh`, `ssh-client`, `terminal`, `tui`
- GitHub Actions: enabled, restricted to GitHub-owned and verified-creator actions, with the required
  `cachix/install-nix-action@*` action explicitly allowed
- Full-length action SHA requirement: enabled
- Default workflow permissions: read-only
- Workflow pull-request approval permission: disabled
- Dependency vulnerability alerts: enabled

## Main Ruleset

Ruleset `Protect main` is active for the default branch with no bypass actors. It blocks deletion and
non-fast-forward pushes, requires pull requests with resolved conversations and zero approvals for the
single-maintainer state, requires branches to be current, and requires the `required` check from the
GitHub Actions integration. GitHub presents that aggregate check under the `CI` workflow.

## Hosted CI

- Push run `34161287040` failed at revision `815ca9b` because a one-millisecond test timeout expired
  before resource acquisition under race-test load.
- The deterministic test fix was pushed as `03fab64`.
- Push run `34162739316` passed for the exact `03fab64` revision.

## Account And Visibility Limitations

- Private Vulnerability Reporting returns HTTP 404 while this repository is private.
- Secret scanning and push protection return HTTP 422 because they are unavailable for this private
  repository under the current account capabilities.
- `scripts/configure-github.sh` now reports the private reporting limitation and continues reversible
  private configuration.
- `scripts/verify-github.sh` accepts and reports these limitations only for private verification;
  public verification still requires Private Vulnerability Reporting. Secret scanning and push
  protection remain visible warnings when the GitHub API reports them unavailable.
- Re-run configuration and public verification immediately after the explicitly authorized visibility
  change so GitHub can enable the public-repository security features.

## Public Transition

- The owner explicitly authorized MIT distribution, public visibility, and release `v0.1.0`.
- Public verification enabled and confirmed Secret Scanning, Push Protection, and Private
  Vulnerability Reporting; the earlier private-account limitations no longer apply.
- Anonymous API access and HTTPS clone passed at revision
  `316f15b8db11152af3df919c47521e777166d7ec`.
- Final release evidence is recorded in `public-release.md`.
