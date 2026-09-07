---

description: "Task list for secure GitHub publication, CI, releases, documentation, and dependency automation"
---

# Tasks: Publicar el proyecto en GitHub

**Input**: Design documents from `/specs/006-publish-github-project/`

**Prerequisites**: plan.md, spec.md, research.md, data-model.md, contracts/github-publication.md, quickstart.md

**Tests**: Automated and manual acceptance tasks are required by the specification, publication contract, and constitution. Write each story's automated tests first and observe the intended failure before implementation.

**Organization**: Tasks are grouped by user story. Repository visibility remains private until the final cross-cutting gate because public exposure depends on every story, while each story is independently testable with local fixtures or a disposable private repository.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it changes different files and has no dependency on another incomplete task
- **[Story]**: Maps work to US1, US2, US3, US4, or US5 from spec.md
- Every task includes exact repository-relative file paths

## Phase 1: Setup (Shared Automation Infrastructure)

**Purpose**: Create shared locations and deterministic tool/version inputs used by all publication stories

- [X] T001 Create the planned `.github/workflows/`, `internal/tools/vulnpolicy/testdata/`, `scripts/`, and `specs/006-publish-github-project/validation/` structure and add the exact Go 1.26.6 release pin in `.github/release.env`
- [X] T002 Add pinned Gitleaks, actionlint, ShellCheck, Renovate, archive, and checksum tooling to the reproducible development environment in `flake.nix` and `flake.lock`
- [X] T003 Add shared fail-fast temporary-directory, command-stub, redaction, and no-side-effect assertions for automation tests in `scripts/testlib.sh`

---

## Phase 2: Foundational (Blocking Security Policy)

**Purpose**: Implement the shared fail-closed vulnerability policy required by CI, releases, and dependency updates

**CRITICAL**: No hosted automation can become authoritative until exception parsing and scanner failures are tested and fail closed.

- [X] T004 Add failing table tests and synthetic streaming-JSON fixtures for no findings, reachable findings, active/expired/stale/duplicate/malformed exceptions, unreachable findings, and scanner/protocol errors in `internal/tools/vulnpolicy/policy_test.go` and `internal/tools/vulnpolicy/testdata/`
- [X] T005 Implement strict exception schema validation, exact reachable-ID matching, expiration, stale-record detection, bounded diagnostics, and fail-closed scanner handling in `internal/tools/vulnpolicy/policy.go`
- [X] T006 Implement the command entry point that consumes `govulncheck` streaming JSON and emits safe warnings/failures in `cmd/vulncheck-policy/main.go`
- [X] T007 Add the reviewed empty/default exception policy and JSON schema documentation in `.vulnerability-exceptions.json` and `specs/006-publish-github-project/contracts/github-publication.md`
- [X] T008 Add failing wrapper tests for pinned/live database selection, command failure propagation, quoted arguments, and secret-safe output in `scripts/check_vulnerabilities_test.sh`
- [X] T009 Implement the shared pinned/live vulnerability scan entry point without broad waivers in `scripts/check-vulnerabilities.sh` and wire its reproducible tools into `flake.nix`

**Checkpoint**: Reachable vulnerabilities block by default; only exact complete unexpired records can pass, and stale or failed analysis cannot be hidden.

---

## Phase 3: User Story 1 - Publish Source Safely (Priority: P1) MVP

**Goal**: Produce a repository tree that is legally and operationally ready for public visibility, with redacted source/history scanning and enforceable one-maintainer governance.

**Independent Test**: In a disposable repository, scan clean/seeded trees and histories, verify MIT/security/contribution metadata, configure mocked repository settings, and prove that a synthetic secret blocks publication without appearing in logs.

### Tests for User Story 1

> Write these tests first and ensure they fail before implementation.

- [X] T010 [P] [US1] Add failing clean-tree, no-history, newly initialized history, imported all-ref history, and redacted synthetic-secret cases in `scripts/prepublish_test.sh`
- [X] T011 [P] [US1] Add failing mocked GitHub settings cases for product name `Orza`, description `A terminal-native SSH client with a TUI`, tagline `Navigate remote systems from your terminal.`, discovery topics, private vulnerability reporting, required CI, zero approvals for one maintainer, force-push/deletion blocking, and unauthenticated visibility checks in `scripts/github_settings_test.sh`
- [X] T012 [P] [US1] Add failing document-contract checks for root license, private security channel, contributor validation instructions, and third-party notice preservation in `scripts/repository_docs_test.sh`

### Implementation for User Story 1

- [X] T013 [P] [US1] Add the canonical MIT terms for project-owned code with 2026 Orza contributors copyright in `LICENSE`
- [X] T014 [P] [US1] Add latest-release/default-branch support scope, private vulnerability reporting, coordinated disclosure, safe report fields, and dependency reachability guidance in `SECURITY.md`
- [X] T015 [P] [US1] Add local environment, required checks, pull-request, sensitive-path, release, and no-real-secret contribution guidance in `CONTRIBUTING.md`
- [X] T016 [P] [US1] Add sole-maintainer ownership for automation, dependency, release, security, and supply-chain paths in `.github/CODEOWNERS`
- [X] T017 [US1] Configure redacted tree/all-ref scanning and explicit absent/imported/new-history transitions in `.gitleaks.toml` and `scripts/prepublish.sh`
- [X] T018 [US1] Implement quoted, least-privilege configuration and verification for product name `Orza`, description `A terminal-native SSH client with a TUI`, tagline `Navigate remote systems from your terminal.`, discovery topics, rulesets, and security settings in `scripts/configure-github.sh` and `scripts/verify-github.sh`

**Checkpoint**: US1 is independently demonstrable in a disposable private repository; no public visibility change has occurred.

---

## Phase 4: User Story 2 - Validate Every Change Automatically (Priority: P2)

**Goal**: Make one stable required CI gate enforce formatting, tests, race, vet, static analysis, vulnerability policy, builds, vendoring, and redacted secret checks for pull requests and `main`.

**Independent Test**: Run the workflow contract locally and in a disposable pull request, inject one controlled failure per category, and verify read-only fork behavior, actionable diagnostics, stable aggregation, and event-specific cancellation.

### Tests for User Story 2

> Write these tests first and ensure they fail before implementation.

- [X] T019 [P] [US2] Add failing static contract cases for required events, no path filters, full-SHA actions, read-only permissions, non-persisted credentials, hosted runners, stable job names, and pull-request-only cancellation in `scripts/ci_workflow_test.sh`
- [X] T020 [P] [US2] Add failing vendor-integrity cases for tidy changes, modified tracked vendor files, and newly generated untracked vendor files in `scripts/vendor_test.sh`
- [X] T021 [P] [US2] Add failing aggregate-gate simulations for controlled format, test, race, vet, static, vulnerability, build, vendor, and secret failures in `scripts/ci_gate_test.sh`

### Implementation for User Story 2

- [X] T022 [US2] Add pull-request, `main`, merge-group, and manual CI orchestration with read-only default permissions and superseded-PR concurrency in `.github/workflows/ci.yml`
- [X] T023 [US2] Reuse `nix flake check --no-update-lock-file --keep-going -L` and expose stable format/test/race/vet/static/vulnerability/build diagnostics in `.github/workflows/ci.yml` and `flake.nix`
- [X] T024 [US2] Add clean-checkout tidy/vendor regeneration, four Linux/Windows release-output builds, and redacted repository secret scanning in `.github/workflows/ci.yml`
- [X] T025 [US2] Add the scheduled/manual live-database policy scan with read-only permissions and independent concurrency in `.github/workflows/vulnerability-monitor.yml`
- [X] T026 [US2] Add an always-evaluated stable aggregate result that fails unless every required dependency job passed or used a valid vulnerability exception in `.github/workflows/ci.yml`

**Checkpoint**: US2 can protect a disposable repository branch and detect one controlled failure in every mandatory category without release permissions or secrets.

---

## Phase 5: User Story 3 - Download a Platform Release (Priority: P3)

**Goal**: Build six correctly configured archives and publish exactly those archives plus one sorted SHA-256 manifest atomically from an eligible stable tag.

**Independent Test**: Use synthetic staged executables and a mocked GitHub CLI to verify exact naming/content/version inputs, tag/main/CI eligibility, every interrupted publication stage, hostile inputs, same-tag concurrency, and draft-first atomicity.

### Tests for User Story 3

> Write these tests first and ensure they fail before implementation.

- [X] T027 [P] [US3] Add failing six-target `orza_v...` archive, `orza`/`orza.exe` executable-name/mode, non-empty file, sorted SHA-256, extra/missing/duplicate asset, and identical linker-version cases in `scripts/release_test.sh`
- [X] T028 [P] [US3] Add failing exact-tag, leading-zero, non-main commit, absent/failed CI, duplicate published release, stale draft, and same-tag concurrency cases in `scripts/release_eligibility_test.sh`
- [X] T029 [P] [US3] Add failing mocked draft creation, upload interruption, draft verification, generated-note, final publication, actionable diagnostic, and shell-metacharacter/no-side-effect cases in `scripts/release_publish_test.sh`

### Implementation for User Story 3

- [X] T030 [US3] Implement deterministic `orza_v...` `.tar.gz`/`.zip` assembly with one root `orza` or `orza.exe` executable per target and exact closed-inventory names in `scripts/package-release.sh`
- [X] T031 [US3] Implement exact seven-asset, archive content, zero-byte, sorted SHA-256, tag, target, release-name, and generated-note verification in `scripts/verify-release.sh`
- [X] T032 [US3] Add exact `vMAJOR.MINOR.PATCH`, `origin/main` ancestry, required-CI, duplicate-release, and non-cancelling same-tag eligibility gates in `.github/workflows/release.yml`
- [X] T033 [US3] Add the six-entry Go 1.26.6 matrix with vendored `-trimpath`/VCS/version builds, Linux-hosted Linux/Windows compilation, and matching native cgo Darwin runners in `.github/workflows/release.yml`
- [X] T034 [US3] Add one-day intermediate artifacts and job-local `contents: write` draft assembly, exact verification, generated notes, and final publication in `.github/workflows/release.yml`

**Checkpoint**: US3 publishes no stable release on any negative path and produces the exact seven-asset contract for an eligible test tag.

---

## Phase 6: User Story 4 - Install and Start Using the Project (Priority: P4)

**Goal**: Make the root documentation sufficient for users on all supported platforms to select, verify, install, use, update, and remove Orza safely.

**Independent Test**: Give only README.md to new users and verify platform selection, checksum/version/help, synthetic local workflow, keyboard navigation, safe exit, and contributor/release discovery within the specified time limits.

### Tests for User Story 4

> Write these tests first and ensure they fail before implementation.

- [X] T035 [P] [US4] Add failing README contract checks for six targets, `orza_v...` release naming, `orza`/`orza.exe` installation/update/removal, SHA-256 commands and limitation, terminal/SSH assumptions, no-color, synthetic examples, safe exit, contribution, and security links in `scripts/readme_test.sh`
- [X] T036 [P] [US4] Create the anonymized 10-participant timing/correctness protocol and empty aggregate record in `specs/006-publish-github-project/validation/readme-study.md`

### Implementation for User Story 4

- [X] T037 [US4] Add GitHub release download selection plus Linux/macOS/Windows checksum, `orza`/`orza.exe` installation, update, and executable-removal instructions without deleting catalog data in `README.md`
- [X] T038 [US4] Align source/Nix build instructions with the required local gate, exact release trigger, Go 1.26.6 release distinction, supported matrix, Darwin cgo Keychain behavior, and compile-only compatibility limit in `README.md`
- [X] T039 [US4] Add concise synthetic quick-start, version/help checks, keyboard-only TUI launch/navigation, no-color, host-trust/credential boundary, cancellation, and safe-exit guidance in `README.md`
- [X] T040 [US4] Link MIT, private security reporting, contribution guidance, CI status, release assets, and dependency policy from the discoverable opening sections of `README.md`

**Checkpoint**: US4 passes the static documentation contract and is ready for the external participant study.

---

## Phase 7: User Story 5 - Keep Dependencies Current (Priority: P5)

**Goal**: Run exactly one mutating weekly Renovate cycle covering Go/vendor, full-SHA Actions, and Nix while capping every update class at five open branches/PRs and forbidding auto-merge.

**Independent Test**: Run dry-run discovery and a deterministic four-week simulation proving the durable week guard, no mutating reruns/overlap, all-class ceilings, ecosystem-local compatible groups, isolated majors, affected-file/diagnostic output, and ordinary CI on proposals.

### Tests for User Story 5

> Write these tests first and ensure they fail before implementation.

- [X] T041 [P] [US5] Add failing Renovate policy cases for all three managers, tidy/vendor updates, full-SHA comments, ecosystem-local compatible groups, isolated majors, three-day age, disabled auto-merge/bypass modes, and five branch/PR ceilings in `scripts/renovate_config_test.sh`
- [X] T042 [P] [US5] Add failing scheduled/manual permission cases for one Monday cron, non-cancelling concurrency, scheduled-only fine-grained token, dry-run manual mode, non-persisted credentials, and redacted logs in `scripts/dependencies_workflow_test.sh`
- [X] T043 [P] [US5] Add failing four-week simulations for atomic `automation/renovate/YYYY-Www` acquisition, overlap/rerun no-op, retained failed-cycle marker, sixth proposal blocking, affected files, and actionable failed-update diagnostics in `scripts/dependencies_cycle_test.sh`

### Implementation for User Story 5

- [X] T044 [US5] Configure Go, GitHub Actions, and beta Nix discovery, post-update tidy/vendor, grouping, major approval, release age, branch/PR ceilings, and disabled automerge/vulnerability PR modes in `.github/renovate.json`
- [X] T045 [US5] Implement the once-weekly trusted Renovate job and read-only manual dry run with durable UTC week-tag guard and least-privilege token isolation in `.github/workflows/dependencies.yml`
- [X] T046 [US5] Add explicit separate handling for `nixpkgs` and `vulndb`, plus a narrowly scoped fallback custom manager when beta Nix discovery cannot update the explicit revision, in `.github/renovate.json`
- [X] T047 [US5] Validate old/new version, affected-file, failed-update diagnostic, ordinary CI, no-automerge, and global-limit behavior in `scripts/dependencies_cycle_test.sh`

**Checkpoint**: US5 covers all dependency sources once per week without duplicate bots, bypass proposals, or unreviewed merges.

---

## Phase 8: Polish & Cross-Cutting Publication

**Purpose**: Validate all stories together, establish Git/GitHub state safely, and perform the authorized public release only after every reversible gate passes

- [ ] T048 Run actionlint, ShellCheck, focused automation tests, formatting, full/race/acceptance tests, vet, build, Staticcheck, pinned/live vulnerability policy, vendor integrity, and `nix flake check` from `specs/006-publish-github-project/quickstart.md`, recording results in `specs/006-publish-github-project/validation/quality-gates.md`
- [ ] T049 Execute the redacted current-tree pre-public scan, detect and separately inspect any Git LFS objects/submodules, and review repository documents, recording findings/remediation and `sourceReview=passed` in `specs/006-publish-github-project/validation/publication-readiness.md`
- [ ] T050 Resolve the authoritative-history decision, initialize a new Git history when none is supplied, rescan every resulting ref plus any imported LFS/submodule history, and record `historyReview=passed` without claiming nonexistent prior history in `specs/006-publish-github-project/validation/publication-readiness.md`
- [ ] T051 Create or connect the private `github.com/pluque01/orza` repository, push the reviewed `main` history, and verify no token or secret appears in repository/workflow output, recording evidence in `specs/006-publish-github-project/validation/github-settings.md`
- [ ] T052 Configure product name `Orza`, description `A terminal-native SSH client with a TUI`, tagline `Navigate remote systems from your terminal.`, discovery topics, full-SHA action policy, dependency/security alerts, private vulnerability reporting, push protection where available, and the one-maintainer `main` ruleset; create a real issue/exception only if a reachable unfixed vulnerability remains, recording account limitations in `specs/006-publish-github-project/validation/github-settings.md`
- [ ] T053 Execute the CI event/failure matrix and Renovate onboarding/dry-run/four-week simulation against disposable inputs, recording stable checks and bounded proposal behavior in `specs/006-publish-github-project/validation/hosted-automation.md`
- [ ] T054 Execute the 10-participant README protocol, record anonymized aggregate outcomes in `specs/006-publish-github-project/validation/readme-study.md`, and complete the pre-public FR-001–FR-028/SC-001–SC-012 acceptance plus constitution compliance review in `specs/006-publish-github-project/validation/acceptance.md`
- [ ] T055 Obtain explicit final owner authorization, switch repository visibility to public, verify anonymous clone/license/README/SECURITY/rules, push eligible `v0.1.0`, verify the immutable seven-asset release plus host-compatible `--version` evidence, and finalize post-public SC-002/SC-003/SC-010 evidence in `specs/006-publish-github-project/validation/public-release.md` and `specs/006-publish-github-project/validation/acceptance.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; creates shared paths and pins.
- **Foundational (Phase 2)**: Depends on Setup and blocks hosted workflows because vulnerability failures must fail closed.
- **US1 (Phase 3)**: Depends on Foundation; establishes reversible publication readiness and governance in fixtures/private scope.
- **US2 (Phase 4)**: Depends on Foundation; can proceed alongside US1 after shared security policy exists.
- **US3 (Phase 5)**: Depends on Foundation; can develop locally in parallel but hosted release acceptance depends on US2's required CI name.
- **US4 (Phase 6)**: Depends on the US3 artifact contract for exact installation names; static documentation work can begin after Foundation.
- **US5 (Phase 7)**: Depends on US2's ordinary PR gate and US1's repository/token policy; config tests can begin after Foundation.
- **Polish/Public Publication (Phase 8)**: Depends on all five stories and must run sequentially because visibility and release operations are externally observable.

### User Story Dependency Graph

```text
Setup -> Foundation -> US1 -----> US5 --+
                    \-> US2 -> US3 -> US4 +-> Final private validation -> Public visibility -> v0.1.0
```

### User Story Independence

- **US1**: Uses disposable repositories and mocked GitHub settings; proves source/history/license/governance readiness without CI or public exposure.
- **US2**: Uses controlled workflow failures and a disposable PR; proves required validation without publishing releases.
- **US3**: Uses synthetic executables and mocked release APIs; proves exact atomic release behavior without a public repository.
- **US4**: Uses static contracts and participant tasks against README; does not require changing application behavior.
- **US5**: Uses Renovate dry-run and deterministic time/API stubs; proves cadence/limits without mutating production during tests.

### Within Each User Story

- Write automated tests first and observe the expected failure.
- Complete policy/data validation before workflow orchestration that consumes it.
- Keep untrusted values in environment variables and quote every command argument.
- Grant write permission only after read-only build/verification jobs succeed.
- Run each independent checkpoint before starting externally visible publication tasks.

### Parallel Opportunities

- T010-T012 can run in parallel because they create separate US1 test files.
- T013-T016 can run in parallel because they create separate root/ownership documents.
- T019-T021 can run in parallel because they validate independent CI contract layers.
- T027-T029 can run in parallel because they cover packaging, eligibility, and publication separately.
- T035-T036 can run in parallel because static README checks and the participant protocol use separate files.
- T041-T043 can run in parallel because config, workflow security, and cycle-state simulations are separate.
- Story-local tests for US1, US2, US3, US4, and US5 can be developed concurrently after Foundation; shared workflow/config files remain sequential.

---

## Parallel Example: User Story 1

```text
Task: "T010 Add pre-public tree/history tests in scripts/prepublish_test.sh"
Task: "T011 Add mocked settings tests in scripts/github_settings_test.sh"
Task: "T012 Add repository document tests in scripts/repository_docs_test.sh"
```

## Parallel Example: User Story 2

```text
Task: "T019 Add CI workflow policy tests in scripts/ci_workflow_test.sh"
Task: "T020 Add tracked/untracked vendor tests in scripts/vendor_test.sh"
Task: "T021 Add aggregate failure simulations in scripts/ci_gate_test.sh"
```

## Parallel Example: User Story 3

```text
Task: "T027 Add archive/checksum/version tests in scripts/release_test.sh"
Task: "T028 Add tag/main/CI eligibility tests in scripts/release_eligibility_test.sh"
Task: "T029 Add draft/failure/hostile-input tests in scripts/release_publish_test.sh"
```

## Parallel Example: User Story 4

```text
Task: "T035 Add README contract tests in scripts/readme_test.sh"
Task: "T036 Create participant protocol in specs/006-publish-github-project/validation/readme-study.md"
```

## Parallel Example: User Story 5

```text
Task: "T041 Add Renovate config tests in scripts/renovate_config_test.sh"
Task: "T042 Add dependency workflow permission tests in scripts/dependencies_workflow_test.sh"
Task: "T043 Add four-week cycle simulations in scripts/dependencies_cycle_test.sh"
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete Setup and the fail-closed vulnerability Foundation.
2. Write and fail T010-T012.
3. Implement T013-T018.
4. Validate a clean and seeded disposable repository.
5. Stop with a demonstrably public-ready but still private source tree; do not switch real visibility yet.

### Incremental Delivery

1. **Foundation**: Deterministic tools and vulnerability exception policy.
2. **US1**: Legal/security/governance readiness in reversible private scope.
3. **US2**: Required CI and current vulnerability monitoring.
4. **US3**: Exact six-target atomic release pipeline.
5. **US4**: Complete installation, usage, security, and contribution documentation.
6. **US5**: Exactly weekly, bounded, reviewed dependency proposals.
7. **Publication**: Scan, initialize/import history, validate privately, configure GitHub, make public, and publish `v0.1.0`.

### Parallel Team Strategy

1. Complete Setup and Foundation together.
2. Develop US1 tests/documents and US2 CI tests in parallel.
3. Develop US3 release scripts and US5 dependency simulations in parallel once shared policy is stable.
4. Finalize US4 against the exact US3 artifact contract.
5. Serialize all Phase 8 operations under the sole maintainer because they mutate Git/GitHub state.

---

## Notes

- `[P]` means different files and no dependency on another incomplete task.
- `[US1]` through `[US5]` provide direct story traceability.
- No task may insert a real secret to test scanning or workflow isolation.
- Full-SHA action references retain readable release comments for automated review.
- Do not enable Dependabot update PRs alongside Renovate.
- Do not run mutating Renovate from manual dispatch or more than once per UTC week marker.
- Do not publish a stable release from a non-main commit, failed/absent CI, or incomplete draft.
- Do not switch visibility before T048-T054 pass and T055 receives explicit final owner authorization.
