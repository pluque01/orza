---

description: "Implementation tasks for safe SSH startup error diagnostics"
---

# Tasks: Motivos de error al iniciar SSH

**Input**: Design documents from `/specs/003-show-ssh-errors/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Required by the feature's controlled failure scenarios and the project constitution. Write each listed test before its corresponding implementation and establish that it fails for the intended reason.

**Organization**: Tasks are grouped by user story so each increment has an explicit goal and independent acceptance boundary.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it edits different files and does not depend on an incomplete task.
- **[Story]**: Maps the task to its user story (`US1`, `US2`, or `US3`).
- Every task names the exact file or files it changes.

## Phase 1: Setup (Requirements Gate)

**Purpose**: Resolve the formal readiness gate before implementation tasks consume the requirements.

- [x] T001 Reconcile the stage vocabulary, 256-Unicode-code-point limit, signal/cancellation boundary, exit-code commitment, structured CLI semantics, and missing-target recovery findings from `specs/003-show-ssh-errors/checklists/readiness.md` into `specs/003-show-ssh-errors/spec.md`, `specs/003-show-ssh-errors/data-model.md`, and `specs/003-show-ssh-errors/contracts/cli.md`, then record each disposition in the checklist

**Checkpoint**: The approved specification and design contracts use one unambiguous vocabulary and no formal-gate conflict remains unresolved.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish the shared safe diagnostic model and captured-target contract used by every user story.

**CRITICAL**: No user story implementation begins until this phase is complete.

### Tests for Shared Foundations

- [x] T002 [P] Define table-driven tests for all `SSHFailureReason`, `SSHFailureStage`, safe presentation, `Error`, `Unwrap`, normalization precedence, and unknown fallback invariants in `internal/app/ssh_start_failure_test.go`
- [x] T003 [P] Define fuzz properties for valid UTF-8, one-line output, control/ANSI/bidi removal, sensitive-pattern rejection, and the 256-Unicode-code-point bound (255 plus `…` when truncated) in `internal/app/ssh_start_failure_fuzz_test.go`
- [x] T004 [P] Define captured-target and pre-active/deadline-versus-cancel regression tests for `ConnectResult` and `ConnectService.Connect` in `internal/app/connect_test.go`

### Implementation for Shared Foundations

- [x] T005 Implement the normative category/stage/summary/recommendation/detail matrix, `SSHStartError`, normalization precedence, allowlisted detail sanitization, 255-plus-`…` truncation, and cause-preserving `Unwrap` in `internal/app/ssh_start_failure.go`
- [x] T006 [P] Add the secret-free `SSHAttemptTarget` snapshot and failure presentation fields to connection/session result contracts in `internal/app/types.go`
- [x] T007 Capture the resolved ID, revision, path, host, and port in every `ConnectService.Connect` result; preserve normalized startup errors; and classify deadline separately from explicit cancellation in `internal/app/connect.go`

**Checkpoint**: All interfaces can consume one cause-free diagnostic and one immutable public target snapshot without importing `internal/sshclient`.

---

## Phase 3: User Story 1 - Understand Why the Connection Failed (Priority: P1) MVP

**Goal**: Classify every controlled pre-active SSH failure into exactly one safe reason and show its target, stage, recommendation, and optional bounded detail in the TUI.

**Independent Test**: Run the controlled ten-category matrix through the application and TUI; each pre-active failure yields the expected category and captured target, authentication denial differs from timeout, unknown causes use the safe fallback, canaries never appear, and post-active remote status remains a session result.

### Tests for User Story 1

- [x] T008 [P] [US1] Add typed DNS resolution, dial, host-verification, handshake, credential, authentication, PTY, shell, terminal, in-app cancellation, deadline, wrapper, process-signal, and joined-cleanup classification cases that also assert every acquired SSH resource closes and terminal state restores in `internal/sshclient/client_test.go`
- [x] T009 [P] [US1] Add Unix errno classification cases for refused and unreachable network failures in `internal/sshclient/network_error_unix_test.go`
- [x] T010 [P] [US1] Add Windows errno classification cases for refused and unreachable network failures in `internal/sshclient/network_error_windows_test.go`
- [x] T011 [P] [US1] Add local agent/key/passphrase/prompt failure versus remote authentication-denial cases in `internal/sshclient/auth_test.go`
- [x] T012 [P] [US1] Extend the in-process SSH server scenario with a real rejected password, deterministic connection cleanup/terminal restoration, and unchanged post-active remote status in `tests/integration/ssh_server_test.go`
- [x] T013 [P] [US1] Add TUI view cases for all categories, captured path/endpoint, always-visible stage/recommendation, hidden detail, fallback, no-color, canary redaction, and sub-second injected completion-to-model-update latency in `internal/tui/errors_test.go`

### Implementation for User Story 1

- [x] T014 [P] [US1] Map typed Unix DNS, timeout, connection-refused, and network-unreachable causes without localized string matching in `internal/sshclient/network_error_unix.go`
- [x] T015 [P] [US1] Map typed Windows DNS, timeout, connection-refused, and network-unreachable causes without localized string matching in `internal/sshclient/network_error_windows.go`
- [x] T016 [US1] Preserve local authentication callback failures and convert exhausted, actually attempted SSH methods into a private authentication-denied sentinel in `internal/sshclient/client.go`
- [x] T017 [US1] Normalize DNS target resolution, network connection, host trust, negotiation, credential, authentication, session-setup, and local-terminal failures into the nine stable `app.SSHFailureStage` values while leaving catalog lookup and post-active stream/cleanup results unchanged in `internal/sshclient/client.go` and `internal/sshclient/session.go`
- [x] T018 [US1] Map agent, identity-file, key-parser, passphrase, and secret-prompt failures to credential-unavailable causes without embedding key paths or parser text in `internal/sshclient/auth.go`
- [x] T019 [US1] Preserve host-trust status, primary normalized cause, target snapshot, and safe unexpected fallback through application wrappers in `internal/app/connect.go`
- [x] T020 [US1] Replace the generic startup error modal with the safe failure projection and captured target, including category, stage, recommendation, and optional hidden detail state in `internal/tui/errors.go`
- [x] T021 [US1] Route only pre-active failures into the diagnostic modal and preserve post-active result/exit behavior in `internal/tui/session.go`

**Checkpoint**: User Story 1 is independently usable as the MVP through the TUI and shared application diagnostic model.

---

## Phase 4: User Story 2 - Recover Without Losing Context (Priority: P2)

**Goal**: Keep the failed target stable and provide keyboard-only back, edit, detail, and explicit retry flows with fresh resolution and confirmation before network I/O.

**Independent Test**: From a pre-active TUI failure, change the browser selection and mutate or delete the catalog target; detail toggling preserves mandatory fields, back/edit use the captured ID, retry performs no SSH I/O before a fresh confirmation, and only an accepted current revision starts another attempt.

### Tests for User Story 2

- [x] T022 [P] [US2] Add immutable attempt snapshot, stale-operation suppression, and pre-active failure transition cases in `internal/tui/session_test.go`
- [x] T023 [P] [US2] Add keyboard-only `d`, `r`, `e`, `Esc`, `q`, canceled confirmation, canceled edit, missing target, catalog reload, changed revision, and no-network-before-confirmation cases in `internal/tui/model_test.go`
- [x] T024 [P] [US2] Add previous/current target confirmation, CAS request, missing/conflict recovery, 80x24, reduced, and no-color cases in `internal/tui/modal_test.go`

### Implementation for User Story 2

- [x] T025 [US2] Add a dedicated detail-toggle binding and modal-scoped help text without changing browser delete behavior in `internal/tui/keys.go` after T023 establishes the failing key-flow contract
- [x] T026 [US2] Extend failure-modal state with immutable attempt target, detail visibility, and explicit idle/resolving/confirming/editing/missing/conflict recovery states in `internal/tui/errors.go`
- [x] T027 [US2] Populate the immutable attempt snapshot before session execution and retain it independently of current browser selection in `internal/tui/session.go`
- [x] T028 [US2] Implement modal-scoped detail, retry, edit, back, and quit event handling by captured connection ID in `internal/tui/model.go`
- [x] T029 [US2] Resolve the current target for retry/edit, display previous/current public targets after changes, and require `y` before constructing a revision-pinned connection request in `internal/tui/modal.go`
- [x] T030 [US2] Keep category, target, stage, recommendation, and recovery controls visible at 80x24/reduced sizes while optional detail yields first in `internal/tui/errors.go`
- [x] T031 [US2] Preserve failed-target context for missing/conflicting retry or edit resolution and offer back or catalog reload while preventing network, persistence, and target substitution in `internal/tui/model.go`

**Checkpoint**: User Story 2 is independently testable against injected diagnostics and catalog changes, with no dependency on CLI presentation.

---

## Phase 5: User Story 3 - Receive the Same Diagnosis in Every Interface (Priority: P3)

**Goal**: Present the same stable category and captured target in TUI, readable CLI, and structured CLI while preserving broad error codes, stderr boundaries, signals, and remote exits.

**Independent Test**: Feed the same controlled causes to all three presenters; category, stage, target, recommendation, and safe optional detail agree, structured failures produce exactly one stderr object, and existing broad process statuses remain unchanged.

### Tests for User Story 3

- [x] T032 [P] [US3] Add readable and JSON startup-diagnostic schema, optional-field omission, stdout/stderr separation, canary redaction, and sub-second injected completion-to-complete-stderr-write cases in `internal/cli/output_test.go`
- [x] T033 [P] [US3] Replace the JSON-connect rejection case with revision-pinned resolution, structured pre-active failure, trust prompt, and no-success-envelope cases in `internal/cli/connect_command_test.go`
- [x] T034 [P] [US3] Add category-to-existing-code/exit mappings plus cancellation, signal, and remote-status regressions in `internal/cli/exit_test.go`
- [x] T035 [P] [US3] Add the ten-category TUI/readable-CLI/structured-CLI parity matrix with captured targets, safe detail canaries, no second network operation, and sub-second completion-to-presentation timing in `tests/integration/ssh_failure_parity_test.go`

### Implementation for User Story 3

- [x] T036 [P] [US3] Extend `ManagementError` with the cause-free startup diagnostic and captured endpoint while preserving `errors.Is`/`errors.As` and existing broad codes in `internal/cli/exit.go`
- [x] T037 [P] [US3] Extend human and JSON error presenters with category, stage, endpoint, recommendation, and optional `technicalDetail`, emitting no raw cause and exactly one JSON object in `internal/cli/output.go`
- [x] T038 [US3] Permit `--json connect`, resolve once to captured ID/revision, preserve trust interaction, and attach the shared startup diagnostic to command errors in `internal/cli/connect.go`
- [x] T039 [US3] Keep structured startup failures on stderr, session/trust I/O on stdout, signal exits 130/143, and remote exit propagation unchanged in `internal/cli/root.go` and `cmd/orza/main.go`
- [x] T040 [US3] Use the shared application projection for readable and structured CLI category/code mapping instead of reclassifying wrapped causes in `internal/cli/connect.go` and `internal/cli/exit.go`
- [x] T041 [US3] Align the TUI labels and CLI human messages with the same controlled summaries and recommendations without changing stable machine identifiers in `internal/tui/errors.go` and `internal/cli/output.go`

**Checkpoint**: All three user-visible interfaces satisfy one stable diagnostic contract without exposing internal causes.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Complete confidentiality, documentation, compatibility, and release gates across all stories.

- [x] T042 [P] Document categories, stages, TUI recovery keys, structured stderr schema, compatibility boundaries, supported platforms, and security limitations in `README.md`
- [x] T043 [P] Add cross-surface canary coverage for views, human/JSON output, returned errors, diagnostics, logs, and catalog persistence using secrets, credential references, server text, wrapped/joined causes, ANSI/control/bidi payloads, and oversized details in `tests/integration/security_test.go`
- [x] T044 Run the bounded sanitizer fuzz campaign from `specs/003-show-ssh-errors/quickstart.md` and convert every reproducible finding into a deterministic regression in `internal/app/ssh_start_failure_fuzz_test.go`
- [x] T045 Run formatting, the complete automated test suite, race detection, vet, staticcheck, govulncheck, and Nix checks; all mandatory gates must pass before merge, with results recorded in `specs/003-show-ssh-errors/quickstart.md`
- [x] T046 Run Windows, Linux, and macOS amd64/arm64 cross-build validation for the platform-specific classifiers and record the results in `specs/003-show-ssh-errors/quickstart.md`
- [x] T047 Document threat assumptions, automated real-SSH/terminal boundary coverage, and any boundary that still requires manual security verification in `specs/003-show-ssh-errors/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Phase 1, Setup**: Starts immediately and gates all code work.
- **Phase 2, Foundational**: Depends on T001 and blocks all user stories.
- **Phase 3, US1**: Depends on T002-T007 and delivers the recommended MVP.
- **Phase 4, US2**: Depends on T002-T007; practical integration tasks T026-T030 consume the US1 modal from T020, but tests and recovery-state work can begin earlier.
- **Phase 5, US3**: Depends on T002-T007; CLI tasks can proceed alongside US1/US2, while parity task T035 completes after T020-T021 and T036-T041.
- **Phase 6, Polish**: Depends on every story selected for release and completes with T042-T047.

### User Story Dependency Graph

```text
T001 Requirements Gate
  -> T002-T007 Shared Foundation
       -> US1 (T008-T021) -> MVP
       -> US2 tests/state (T022-T025)
             -> US1 T020 -> US2 integration (T026-T031)
       -> US3 CLI (T032-T034, T036-T040)
             -> US1 presentation + US2 recovery -> parity T035/T041
  -> Selected stories -> T042-T047 Release Gates
```

### Within Each User Story

- Write the listed tests first and establish the intended failure before implementation.
- Implement typed models and classification before presentation.
- Implement captured-target state before recovery actions.
- Implement CLI diagnostic transport before formatting and cross-interface parity.
- Complete each story checkpoint before treating that story as releasable.

### Parallel Opportunities

- T002, T003, and T004 can run in parallel; T006 can proceed alongside T005 after test contracts are understood.
- US1 test tasks T008-T013 can run in parallel in distinct files.
- T014 and T015 can run in parallel after T009/T010; T012 and T013 remain independent of those platform files.
- US2 test tasks T022-T024 can run in parallel; T025 starts after T023 establishes the failing key-flow contract.
- US3 test tasks T032-T035 must be authored before implementation and can run in parallel; they are expected to pass only after T036-T041 complete.
- T042 and T043 can run in parallel after feature behavior stabilizes.

---

## Parallel Examples

### User Story 1

```text
Task T008: Add lifecycle classification tests in internal/sshclient/client_test.go
Task T009: Add Unix errno tests in internal/sshclient/network_error_unix_test.go
Task T010: Add Windows errno tests in internal/sshclient/network_error_windows_test.go
Task T011: Add authentication-boundary tests in internal/sshclient/auth_test.go
Task T012: Add real rejection coverage in tests/integration/ssh_server_test.go
Task T013: Add diagnostic view coverage in internal/tui/errors_test.go
```

### User Story 2

```text
Task T022: Add attempt-state tests in internal/tui/session_test.go
Task T023: Add recovery key-flow tests in internal/tui/model_test.go
Task T024: Add confirmation/layout tests in internal/tui/modal_test.go
Task T025: Add modal detail binding in internal/tui/keys.go
```

### User Story 3

```text
Task T032: Add CLI presenter contract tests in internal/cli/output_test.go
Task T033: Add connect JSON contract tests in internal/cli/connect_command_test.go
Task T034: Add process-exit mapping tests in internal/cli/exit_test.go
Task T035: Add interface parity tests in tests/integration/ssh_failure_parity_test.go
```

---

## Implementation Strategy

### MVP First: User Story 1

1. Complete T001 to close the requirements gate.
2. Complete T002-T007 to establish the shared safe model.
3. Complete T008-T021 to deliver classified TUI startup diagnostics.
4. Stop at the US1 checkpoint and validate the ten-category matrix, confidentiality, and post-active regression independently.

### Incremental Delivery

1. **Foundation**: Requirements gate plus typed safe diagnostic and immutable target.
2. **MVP**: US1 classification and TUI presentation.
3. **Recovery**: US2 keyboard recovery and revision-safe retry.
4. **Parity**: US3 readable/structured CLI and cross-interface consistency.
5. **Release**: Documentation, canaries, fuzzing, quality gates, cross-builds, and security-review evidence.

### Parallel Team Strategy

1. Complete T001-T007 together.
2. Assign SSH classification/TUI presentation to the US1 owner.
3. Start US2 state/tests and US3 CLI tests in parallel after the shared model stabilizes.
4. Integrate US2 after T020 and complete US3 parity after both presentation paths exist.

## Notes

- `[P]` tasks edit distinct files and have no dependency on unfinished tasks in the same wave.
- Tests are intentionally included because this feature changes security-sensitive SSH lifecycle, input handling, terminal recovery, and user-visible failure paths.
- No task may expose or persist an arbitrary wrapped cause merely to provide technical detail.
- Stop at any checkpoint to validate that increment independently.

---

## Phase 7: Convergence

- [x] T048 Classify an available SSH agent with zero identities as `credential_unavailable` with safe `agent` detail instead of `authentication_denied`, preserve genuine remote rejection, and add direct plus in-process SSH regressions in `internal/sshclient/auth.go`, `internal/sshclient/auth_test.go`, `internal/sshclient/client_test.go`, and `tests/integration/ssh_server_test.go` per FR-002 and Edge Cases (partial)
