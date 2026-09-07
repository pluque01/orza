# Quickstart Validation: Publicar el proyecto en GitHub

## Purpose

Use this guide after implementation to prove the repository is safe to expose, required checks are reproducible, dependency proposals are bounded, and a stable tag can publish exactly six platform archives plus one checksum manifest without exposing a partial release.

## Prerequisites

- Repository root: `/home/fallen/code/orza`
- Nix with flakes enabled
- Git history initialized here or imported from its authoritative source
- GitHub CLI authenticated as an administrator of `github.com/pluque01/orza`
- Permission to create/configure the public repository, rulesets, private vulnerability reporting, and releases
- A temporary private repository or non-stable test tag for destructive workflow validation before first publication
- Synthetic fixtures only; no production host, credential, private key, passphrase, or real fingerprint is required

The current workspace had no `.git` directory during planning. Do not mark history review as passed until implementation either imports and scans the intended history or explicitly records `not_present`, scans the complete tree, and initializes a new history afterward.

## Contract References

- [Specification](spec.md): approved user behavior and success criteria
- [Research](research.md): tool and security decisions
- [Data model](data-model.md): validation, exception, release, and proposal states
- [GitHub publication contract](contracts/github-publication.md): exact event, asset, policy, and documentation behavior

## Local Quality Gate

Enter the pinned environment and run the same required validation used by CI:

```sh
nix develop
nix flake check --no-update-lock-file --keep-going -L
```

Validate that the committed Go dependency graph regenerates without change:

```sh
test -z "$(git status --porcelain=v1 --untracked-files=all)"
go mod tidy -diff
go mod vendor
test -z "$(git status --porcelain=v1 --untracked-files=all -- go.mod go.sum vendor)"
```

Run focused automation tests and static workflow/script checks provided by the implementation:

```sh
go test ./internal/tools/vulnpolicy
bash scripts/release_test.sh
actionlint .github/workflows/*.yml
shellcheck scripts/*.sh
```

Expected outcomes:

- Formatting, full tests, race, vet, Staticcheck, vulnerability policy, build, and native package smoke checks pass.
- Regenerating modules changes no tracked file.
- Workflow expressions and shell entry points pass static validation.
- No validation downloads an unreviewed Go toolchain or reads real SSH/credential data.

## Vulnerability Policy

Run the policy against the repository-locked database:

```sh
scripts/check-vulnerabilities.sh pinned
```

Run the monitoring form against current vulnerability data:

```sh
scripts/check-vulnerabilities.sh live
```

Test the policy package with synthetic scanner streams covering:

```sh
go test ./internal/tools/vulnpolicy -run 'Test(Active|Expired|Stale|Unknown|Malformed|ScannerError)'
```

Expected outcomes:

- No reachable finding passes.
- An exact, complete, unexpired exception emits a visible warning and passes.
- Unknown findings, malformed records, expired records, stale records, and scanner/protocol errors fail.
- `.vulnerability-exceptions.json` contains no wildcard, module-wide, severity-wide, or command-wide waiver.
- Any temporary `GO-2026-5972` record has a real issue URL, owner, mitigation, and near-term expiration; it is removed when the locked Nix toolchain reaches Go 1.26.6 or newer.

## Pre-Publication Security Gate

Scan the complete working tree with redacted output:

```sh
gitleaks dir --redact .
```

When Git history exists, fetch all intended refs and scan them:

```sh
git fetch --all --tags --prune
gitleaks git --redact --log-opts="--all"
```

Also review Git LFS objects and submodules if either is introduced. For a controlled negative test, use only a scanner-provided synthetic test token in a disposable branch; confirm the scan fails and the log does not print the token value.

Before continuing:

1. Revoke or rotate every credible credential finding.
2. Remove the finding from the current tree and any history intended for publication.
3. Repeat both scans.
4. Record whether history was genuinely `not_present` before initialization.
5. After importing or creating Git history, scan all resulting refs and record `historyReview=passed` before visibility changes.
6. Confirm root MIT, README, SECURITY, CONTRIBUTING, and ownership files contain no personal secret or production endpoint.

## CI Event Validation

Open or update a pull request and inspect the stable aggregate check:

```sh
gh pr checks
```

Confirm the same workflow runs after merge to `main`, can be dispatched manually, and supports `merge_group` if a merge queue is later enabled. Inspect workflow permissions through the repository UI or API.

Acceptance matrix:

| Case | Expected result |
|------|-----------------|
| Clean pull request | Stable required gate passes |
| Controlled format failure | Gate fails with check and reproduction command |
| Controlled test/race/vet/static failure | Corresponding result fails and aggregate blocks |
| Controlled compile failure | Build result fails and aggregate blocks |
| Vendor regeneration difference | Vendor result fails |
| Unknown reachable vulnerability | Vulnerability result fails |
| Synthetic secret fixture | Secret result fails and redacts the fixture value |
| Fork pull request | Runs read-only without repository secrets |
| New commit on same pull request | Older pull-request run may cancel; newest remains authoritative |
| Main or manual run | Is not cancelled merely by a later run |

Do not use a real secret to test redaction or fork isolation.

## Release Assembly Validation

Validate packaging and verification with the test script, which creates six synthetic staged executables and a disposable output directory without requiring local Darwin builds:

```sh
bash scripts/release_test.sh
```

The script exercises assembly and verification only; it must never publish its fixtures.

Expected release inventory for `v1.2.3`:

```text
orza_v1.2.3_linux_amd64.tar.gz
orza_v1.2.3_linux_arm64.tar.gz
orza_v1.2.3_darwin_amd64.tar.gz
orza_v1.2.3_darwin_arm64.tar.gz
orza_v1.2.3_windows_amd64.zip
orza_v1.2.3_windows_arm64.zip
orza_v1.2.3_checksums.txt
```

Verify:

- Linux and Windows targets compile on the Linux builder with cgo disabled.
- Darwin amd64 and arm64 compile on matching native macOS runners with cgo enabled.
- Every archive is non-empty and contains exactly one correctly named executable.
- The checksum manifest is sorted by filename and validates all six final archives.
- Build jobs cannot create or modify a release.
- The compile-only gate does not claim native runtime compatibility.
- Every matrix build receives the same validated `0.0.0` linker version in the synthetic command-capture test.
- A host-native post-public check can execute its downloaded artifact with `--version`; this evidence does not become a six-target publication gate.

## Draft-First Release Test

Use a temporary private repository or a deliberately non-stable workflow-dispatch dry run before the first real tag. Exercise these negative cases without publishing:

1. Invalid tag syntax such as `1.2.3`, `v01.2.3`, or `v1.2`.
2. Valid-looking tag whose commit is not contained in `origin/main`.
3. Exact valid tag on a `main` commit whose required CI gate is failed or absent.
4. One failed target build.
5. Missing, extra, duplicate, or zero-byte archive.
6. Checksum generation failure.
7. Draft creation failure.
8. Interrupted upload.
9. Draft tag/target/asset verification failure.
10. Generated-release-notes failure or body without version/change information.
11. Final draft-to-public transition failure.
12. Existing published release for the same tag.
13. Two same-tag runs.
14. Hostile tag, path, environment, and release-note values containing spaces, control characters, shell substitutions, quotes, separators, and option prefixes.

Every case must leave zero stable public release and identify the failed stage/target, cause, and evidence or reproduction location. Hostile values must be rejected or handled as inert data with no side effect beyond the disposable test directory. A stale draft may be removed only after confirming that no published release exists.

For the first stable test tag on `main`:

```sh
git tag v0.1.0
git push origin v0.1.0
gh run watch
gh release view v0.1.0 --json isDraft,tagName,targetCommitish,name,body,assets
```

Confirm the release changes from draft to public only after the exact seven-asset inventory is verified. Confirm its name/body identify `v0.1.0` and include generated changes. Download the assets into a clean directory, verify checksums using the platform's documented SHA-256 command, and run `--version` only on a host-compatible sample as post-public evidence. Do not execute all six binaries as a publication gate; the specification requires compilation only.

## Dependency Automation Validation

Validate `.github/renovate.json`, configure its fine-grained repository token with metadata read plus Contents, Pull requests, Issues, and Workflows read/write only for this repository, and run the Nix-pinned Renovate CLI in dry-run discovery before enabling the once-weekly schedule. Confirm the workflow has exactly one weekly cron trigger, one non-cancelling concurrency group, an atomic `automation/renovate/YYYY-Www` marker, scheduled-job-only secret injection, non-persisted checkout credentials, masked logs, and manual dispatch forced to full dry-run mode with no write token or mutation.

Confirm discovery includes:

- `go.mod`, `go.sum`, and `vendor/`
- Full-SHA references in `.github/workflows/`
- `nixpkgs` and vulnerability-database inputs/locks

Acceptance cases:

| Case | Expected result |
|------|-----------------|
| Compatible Go updates | One ecosystem-local group; tidy/vendor changes included |
| Compatible Actions updates | Full SHA and readable version comment updated together |
| Major update | Separate and waits for dependency-dashboard approval |
| Nix toolchain update | Separate from vulnerability database update |
| Five branches/PRs of any update class already open | Every sixth proposal waits |
| Security vulnerability alert | Remains visible but creates no bypass PR outside the weekly capped cycle |
| Successful proposal | Lists old/new versions and every affected file |
| Deliberately failed update dry run | Identifies dependency, cause, and evidence/reproduction location |
| Any update PR | Ordinary CI runs; no automatic merge |

If the beta Nix manager does not recognize the explicit vulnerability-database revision, stop and add the narrowly scoped reviewed updater described in `research.md`; do not silently omit the dependency.

Exercise a deterministic four-week schedule simulation, or observe and record four consecutive real Monday cycles, proving exactly one mutating invocation per week, never more than five open branches/PRs, no major mixed into a compatible group, and no automatic merge. For each week, attempt overlapping and rerun executions and verify the existing week marker makes them read-only/no-op. Simulate a failed first run and confirm its retained marker blocks a second mutating attempt. The simulation controls time and updater responses without contacting production repositories.

## Documentation Study

Give only the root README to 10 participants who did not implement this feature. Use at least one Linux, one macOS, and one Windows path across the group. Ask each participant to:

1. Select the correct release asset for a stated OS/architecture.
2. Install it using the documented method.
3. Verify the checksum limitation and reported version.
4. Open help and identify how to start and safely exit the TUI.
5. Locate the local quality command and release trigger.

At least 9 of 10 must finish installation, version, and help within 10 minutes. Each must identify all six release destinations, the `vMAJOR.MINOR.PATCH` trigger, and a blocking validation within 15 minutes. Record only timings, correctness, platform path, and aggregate outcome; record no personal data.

## Public Repository Settings

After workflows have stable names and before switching visibility:

1. Configure product name `Orza`, description `A terminal-native SSH client with a TUI`, tagline `Navigate remote systems from your terminal.`, and discovery topics.
2. Configure the `main` ruleset with required pull requests, stable CI, resolved conversations, no force pushes, and no deletion.
3. Set required approvals to zero while there is one maintainer; document changing it to one plus code-owner review when a second maintainer joins.
4. Enable dependency graph/alerts, private vulnerability reporting, and available push protection.
5. Restrict allowed actions and require full-length action SHA pins where account capabilities permit.
6. Verify the settings through authenticated API output without printing tokens.
7. Obtain explicit owner authorization, then change visibility to public.
8. From an unauthenticated session, verify clone access, MIT, README, SECURITY, and required-check visibility.

The final handoff records settings that could not be enabled because of account/plan limitations; it must not silently claim compliance.

## Final Gate

The feature is ready only when:

- All local and hosted checks pass.
- The pre-public tree/history decision and scans are recorded.
- No unresolved reachable vulnerability exists outside a valid temporary exception.
- The release negative matrix produces no partial stable release.
- One stable tag publishes exactly the expected seven assets from a `main` commit.
- Renovate discovers all three ecosystems, honors the five-PR limit, and does not auto-merge.
- README study criteria pass.
- Authenticated and unauthenticated repository-setting checks pass or an explicit platform limitation blocks publication.
