# Pre-Public Acceptance Review

**Date:** 2026-09-08
**Repository:** `github.com/pluque01/orza`
**Visibility:** private
**Decision:** ready to request final owner authorization, with human usability measurements waived and
public/release evidence still pending

## Owner Waiver

The repository owner directed the project to skip the 10-participant README study. No sessions were
run and no participant outcome is claimed. SC-005 and the timed-newcomer portion of SC-009 are
therefore `waived/unverified`, not passed. Static README contracts and synthetic safety checks passed.

## Functional Requirements

| Requirement | Pre-public disposition | Evidence or remaining condition |
|---|---|---|
| FR-001 | pass | CI covers pull requests, `main`, merge queue, and manual execution. |
| FR-002 | pass | Formatting, full tests, race, static analysis, vulnerability analysis, and builds pass through the Nix gate. |
| FR-003 | pass | The stable `required` aggregate fails closed with category diagnostics. |
| FR-004 | pass | Workflows use read-only defaults, credentialless checkout, redaction, and full-SHA action pins. |
| FR-005 | pass | Tests and documentation use synthetic local data without real SSH infrastructure. |
| FR-006 | ready, public evidence pending | Release eligibility enforces exact SemVer, `main` ancestry, successful CI, and uniqueness. |
| FR-007 | ready, public evidence pending | The release matrix defines all six targets; native Darwin builds run only in the release workflow. |
| FR-008 | ready, public evidence pending | Packaging tests verify unambiguous archive and executable names. |
| FR-009 | ready, public evidence pending | Version injection and `--version` behavior are tested; downloaded release evidence remains. |
| FR-010 | ready, public evidence pending | Tests verify six checksummed archives and non-empty generated release notes. |
| FR-011 | pass in simulation | Draft-first failure tests prevent a partial stable release. |
| FR-012 | pass in simulation | Same-tag concurrency, immutable existing releases, and bounded failure diagnostics are tested. |
| FR-013 | pass | README purpose, status, support, requirements, integrity, lifecycle, and security contracts pass. |
| FR-014 | pass | README covers release, source, and reproducible Nix installation. |
| FR-015 | pass | README covers help/version, local catalog operations, TUI navigation, cancellation, and exit. |
| FR-016 | pass | README documents checksum limits, host identity, credentials, file permissions, and no-color safety. |
| FR-017 | pass | CONTRIBUTING documents the local gate and stable release trigger. |
| FR-018 | pass for configuration/dry-run | One weekly Renovate schedule is configured; hosted dry-run extracted 72 dependencies. |
| FR-019 | pass | Dependency proposals use ordinary CI, ecosystem-local grouping, isolated majors, and no automerge. |
| FR-020 | pass in simulation | Durable weekly markers, five-proposal ceilings, affected files, and diagnostics passed four-week simulation. |
| FR-021 | pass | Go, Nix, vulnerability data, Actions, and release inputs are visibly pinned. |
| FR-022 | pass | Publication work does not alter SSH controls, persisted formats, or application behavior beyond version output. |
| FR-023 | pass | Redacted tree/all-ref scans and manual LFS/submodule/publication review found no blockers. |
| FR-024 | ready, public evidence pending | Description, topics, README, SECURITY, and MIT license exist; public vulnerability reporting must be verified after visibility changes. |
| FR-025 | pass for one maintainer | Active no-bypass rules require pull requests, current branches, and the protected `required` check. |
| FR-026 | pass | Hostile tag, path, repository, filename, and environment cases are tested. |
| FR-027 | pending explicit authorization | This review does not authorize public visibility or artifact distribution. |
| FR-028 | pass | Complete active exceptions pass; expired, incomplete, stale, and already-fixable exceptions fail closed. |

## Success Criteria

| Criterion | Disposition | Evidence or remaining condition |
|---|---|---|
| SC-001 | pass | Local failure matrices and hosted protected PRs prove all required categories gate integration. |
| SC-002 | public release pending | Deterministic tests produce six archives plus one checksum manifest. |
| SC-003 | public release pending | All target outputs are checked as non-empty; real native Darwin delivery remains. |
| SC-004 | pass in simulation | Every simulated publication-stage interruption leaves no stable partial release. |
| SC-005 | waived/unverified | The owner waived the unrun 10-participant study; no 9-of-10 result is claimed. |
| SC-006 | pass | Documentation tests require synthetic examples and visible cancellation/exit. |
| SC-007 | pass in simulation/dry-run | Discovery works and proposal policy is tested; no mutating production proposal was created. |
| SC-008 | pass in simulation | Four deterministic weekly cycles enforce grouping and the five-proposal ceiling. |
| SC-009 | static portion pass; timed portion waived | Required gate, trigger, targets, and blocker are documented; no timed newcomer result is claimed. |
| SC-010 | public verification pending | Anonymous visibility/clone and public security settings require the authorized visibility change. |
| SC-011 | pass | Current tree and complete new history contain no detected publication blockers. |
| SC-012 | pass | Active, expired, and already-fixable exception behavior is covered by automated tests. |

## Constitution Review

- Secure SSH operations: pass. No SSH trust, credential, protocol, or persisted-state behavior changed.
- Terminal-first usability: implementation and documentation tests pass; human timing evidence is waived.
- Predictable state and failure handling: pass. CI, Renovate, vulnerability, and release paths fail closed
  with bounded diagnostics; real release recovery remains part of final publication verification.
- Behavior-focused testing: pass. Changed automation behavior and principal failure paths have regression
  contracts and hosted evidence.
- Simplicity and maintainability: pass. Existing Go, Nix, GitHub Actions, and Renovate mechanisms are
  reused without a new service or persistent application state.
- Security and operational constraints: pass pre-public. Untrusted inputs, least privilege, action pins,
  redaction, and documented platform assumptions are covered.

No constitutional exception is granted by the study waiver, and no release or visibility change is
authorized by this review. Final public-only checks remain assigned to T055.
