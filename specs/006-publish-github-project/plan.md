# Implementation Plan: Publicar el proyecto en GitHub

**Branch**: `006-publish-github-project` | **Date**: 2026-09-04 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/006-publish-github-project/spec.md`

## Summary

Prepare Orza for safe public distribution at `github.com/pluque01/orza` through GitHub with least-privilege CI, vulnerability-monitoring, dependency-update, and atomic tag-driven release workflows; repository security and contribution metadata; complete installation and usage documentation; and automated dependency proposals. Preserve Nix as the reproducible local and CI validation contract; build release binaries directly with a pinned Go toolchain so native cgo-enabled Darwin builds use Keychain correctly. Publish six archives plus one sorted SHA-256 manifest from a draft release only after all builds succeed. Run a Nix-pinned Renovate command exactly once in a weekly workflow for Go modules, vendored dependencies, GitHub Actions, and Nix inputs, with immutable action pins and a five-branch/five-PR ceiling that no proposal class bypasses.

## Technical Context

**Language/Version**: Go 1.26 module; the reproducible environment and every release builder use exactly Go 1.26.6 with `GOTOOLCHAIN=local` until a reviewed repository change updates that pin everywhere

**Primary Dependencies**: Existing Bubble Tea v2/Lip Gloss v2 application; Nix flakes for reproducible validation; GitHub Actions and GitHub CLI for CI/releases; Nix-pinned Renovate CLI plus a fine-grained repository token for dependency proposals; Gitleaks for pre-public history/tree review; actionlint and ShellCheck for automation validation; `govulncheck` plus a small standard-library policy wrapper for temporary exceptions

**Storage**: Existing local SQLite catalog is unchanged; new persisted project data is repository configuration, release archives/checksums, Renovate policy, and vulnerability-exception JSON

**Testing**: Existing Go unit/integration/race/acceptance suites; `go vet`; Staticcheck; pinned and live-database `govulncheck`; Nix flake checks; workflow syntax/policy tests; release inventory/archive/checksum tests; vulnerability-policy unit tests; manual authenticated repository-settings verification

**Target Platform**: Public GitHub repository; release artifacts for Linux, macOS, and Windows on amd64 and arm64; GitHub-hosted Linux and native Intel/Apple-Silicon macOS runners

**Project Type**: Single-module local CLI/TUI application with repository automation and release tooling

**Performance Goals**: Preserve existing application performance gates; cancel superseded pull-request CI runs; complete routine CI within 30 minutes under normal hosted-runner/cache availability; dependency review runs once in its weekly window

**Constraints**: Releases only from exact `vMAJOR.MINOR.PATCH` tags whose commit is contained in `main`; compile-only six-target release gate; no partial public releases; no auto-merge; maximum five open Renovate PRs; external actions pinned to full commit SHAs; default workflow permissions read-only; no production hosts, credentials, or persistent self-hosted runners; current workspace has no Git metadata and cannot claim a clean history until history is imported or a new repository is initialized

**Scale/Scope**: Four workflows, six release targets, seven release assets, one dependency configuration, one exception policy and checker, root MIT/security/contribution documents, README expansion, and authenticated GitHub repository/ruleset configuration for one maintainer

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Research Gate

| Principle / Constraint | Evaluation | Result |
|------------------------|------------|--------|
| I. Secure SSH Operations | Automation uses synthetic data, scans source/history before exposure, redacts findings, grants no secrets to untrusted PRs, and leaves SSH behavior and credential handling unchanged. | PASS |
| II. Terminal-First Usability | README retains keyboard, minimum terminal, no-color, cancel/back/quit, platform, and installation guidance; no application interaction changes. | PASS |
| III. Predictable State and Failure Handling | CI and release stages fail closed with bounded diagnostics; draft-first publication prevents stable partial releases; duplicate tag runs are serialized. | PASS |
| IV. Behavior-Focused Testing | Existing behavioral suites remain mandatory; policy parsing, release inventory, tag/main eligibility, checksums, vendoring, and failure paths gain automated coverage. | PASS |
| V. Simplicity and Maintainability | Existing Nix checks are reused; direct Go release builds avoid an unnecessary release framework; the only new helper exists because `govulncheck` has no native expiring-exception mechanism. | PASS |
| Untrusted input and bounded output | Tags and GitHub context enter scripts through quoted environment variables, release inventory is allowlisted, logs are bounded, and secret findings are redacted. | PASS |
| Supported platforms and documentation | README will state all six destinations, cgo/credential-store consequences, terminal assumptions, checksum limits, installation, update, and removal. | PASS |
| Workflow and quality gates | Formatting, full tests, race, vet, static analysis, vulnerability analysis, build, documentation, and constitutional review remain pre-merge requirements. | PASS |

No constitutional violations are required. A vulnerability exception is not a silent waiver: it must carry the ID, owner, rationale, issue, mitigation, and expiration/review date, and must fail closed when stale or expired.

### Post-Design Gate

Phase 1 preserves every pre-research result. The release contract grants write permission only to the final publication job, uses native cgo Darwin compilation, and publishes only an exact verified inventory from a draft. The CI contract executes untrusted pull-request code only on ephemeral hosted runners with read-only permissions. The dependency contract disables automatic merging and isolates major updates. The publication checklist does not claim historical safety while `.git` is absent and requires an authenticated post-public settings check. The data model adds only repository policy and derived automation state; application storage and SSH lifecycle remain unchanged. **Result: PASS.**

## Project Structure

### Documentation (this feature)

```text
specs/006-publish-github-project/
├── spec.md
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── github-publication.md
└── tasks.md                      # Future `/speckit.tasks` output
```

### Source Code (repository root)

```text
.github/
├── workflows/
│   ├── ci.yml                     # PR/main validation and stable required gate
│   ├── release.yml                # Six-target draft-first stable release
│   ├── vulnerability-monitor.yml  # Scheduled live-database vulnerability review
│   └── dependencies.yml           # Exactly one weekly Renovate invocation
├── CODEOWNERS                     # Sensitive automation ownership
└── renovate.json                  # Weekly Go/Actions/Nix update policy

cmd/
└── vulncheck-policy/
    └── main.go                    # Streaming govulncheck policy command

internal/tools/vulnpolicy/
├── policy.go                      # Exception validation and finding decisions
├── policy_test.go                 # Active/expired/stale/error regression matrix
└── testdata/                      # Synthetic govulncheck protocol fixtures

scripts/
├── check-vulnerabilities.sh       # Shared pinned/live scan entry point
├── package-release.sh             # Quoted deterministic archive/checksum assembly
├── verify-release.sh              # Exact asset/version/checksum contract checks
└── release_test.sh                # Synthetic packaging and failure matrix

.gitleaks.toml                     # Reviewed secret-scan configuration
.vulnerability-exceptions.json    # Expiring reachable-vulnerability exceptions
LICENSE                            # MIT license for project-owned code
SECURITY.md                        # Private reporting and supported-version policy
CONTRIBUTING.md                    # Local checks and release contribution workflow
README.md                          # Releases, install, verify, use, update, remove
flake.nix                          # Shared tools/check entry points as needed
flake.lock                         # Reviewed toolchain and vulnerability-data pins
go.mod / go.sum / vendor/          # Reproducible Go dependency graph
```

**Structure Decision**: Keep one Go module and the existing Nix flake. Repository automation lives under `.github`; small shell entry points only coordinate established tools and quote all input; exception semantics live in a tested internal Go package rather than fragile output matching. No release framework, package-manager publishing, application updater, telemetry, or application persistence is added.

## Implementation Sequence

1. Add MIT, security, contribution, ownership, and pre-public secret-review files; establish the clean-tree/history decision before any visibility change.
2. Implement and test the expiring vulnerability-exception policy, then make pinned and live scans use the same fail-closed wrapper.
3. Add stable-name CI jobs for pull requests, `main`, manual runs, and merge groups, reusing `nix flake check` and validating vendor regeneration plus all Linux/Windows outputs.
4. Add release packaging/verification scripts and contract tests before wiring the six-target tag workflow.
5. Add draft-first publication with exact tag syntax, `main` ancestry, duplicate protection, sorted SHA-256 manifest, exact seven-asset inventory, and job-local write permission.
6. Configure one weekly Renovate workflow for Go, vendoring, Actions SHAs, and Nix; acquire a durable UTC week tag before mutation, serialize runs, isolate majors, disable auto-merge and vulnerability-alert PR bypasses, and cap every branch/proposal class at five.
7. Expand README with release installation, checksum verification and limitation, source/Nix build, update/removal, keyboard-first use, platform/credential-store constraints, and local quality gates.
8. Validate workflows and scripts locally where possible, run all repository gates, then perform authenticated GitHub visibility, ruleset, security-feature, metadata, and dry-run release checks.

## Complexity Tracking

No constitutional violations or unjustified complexity exceptions are required. The vulnerability-policy helper is the smallest reviewable way to support FR-028 because the upstream scanner does not provide expiring per-ID suppressions and machine-readable findings must be parsed without disabling unrelated failures.
