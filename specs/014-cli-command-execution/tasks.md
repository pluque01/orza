---

description: "Implementation tasks for CLI Command Execution"
---

# Tasks: CLI Command Execution

**Input**: Design documents from `specs/014-cli-command-execution/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/exec.md`, `quickstart.md`

**Tests**: Automated coverage is required by the project constitution for command parsing, connection state, input handling, terminal cleanup, and principal failure paths. Write each listed test before its implementation task and verify it fails for the expected missing behavior.

**Organization**: Tasks are grouped by user story so P1 can be delivered and tested as the MVP before P2 failure hardening.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel with other marked tasks once their stated prerequisites are complete.
- **[US1]** and **[US2]**: Map to the corresponding user story in `spec.md`.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish the feature's test and documentation boundaries without changing catalog persistence.

- [X] T001 [P] Add exec request fixtures and fake command-runner helpers in `internal/app/command_test.go`
- [X] T002 [P] Add exec protocol request support to the in-process test server in `tests/integration/ssh_server_test.go`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Add the transient values, ports, and shared non-interactive seams required by both user stories.

**⚠️ CRITICAL**: Complete this phase before implementation work for either story.

- [X] T003 [P] Define transient SSH command request/result values, including a non-empty command, expected revision, and streamed I/O, in `internal/app/types.go`
- [X] T004 [P] Add the SSH command-runner port and command-service dependency field in `internal/app/ports.go` and `internal/app/bootstrap.go`
- [X] T005 Extend the SSH transport and remote-session interfaces with non-PTY command execution in `internal/sshclient/session.go`
- [X] T006 Generalize SSH authentication setup so the command request preserves host-before-authentication ordering in `internal/sshclient/client.go`
- [X] T007 Wire the command service dependency into root construction in `internal/cli/root.go` and `cmd/orza/main.go`

**Checkpoint**: The application can represent and inject a command operation, but no user-visible command is registered yet.

---

## Phase 3: User Story 1 - Execute a Remote Command from a Script (Priority: P1) 🎯 MVP

**Goal**: A script or agent can execute one remote shell-text command on a saved connection without an interactive terminal, forwarding stdin and streaming stdout/stderr while preserving remote exit status.

**Independent Test**: With a reachable trusted saved connection and no interactive terminal, execute shell text that writes both output channels, accepts piped stdin, and exits with status `23`; verify exact command forwarding, output separation, and process status `23`.

### Tests for User Story 1

- [X] T008 [P] [US1] Add CLI contract tests for `orza exec PATH_OR_ID [--timeout DURATION] -- COMMAND`, command validation, default timeout, `--json` rejection, stream wiring, and remote status propagation in `internal/cli/exec_test.go`
- [X] T009 [P] [US1] Add command-service tests for selector resolution, captured ID/revision, exact remote shell text, and unchanged saved connections in `internal/app/command_test.go`
- [X] T010 [P] [US1] Add non-PTY SSH runner tests for exact exec text, direct stdin/stdout/stderr wiring, no terminal calls, and remote exit status `1` through `255` in `internal/sshclient/session_test.go`
- [X] T011 [P] [US1] Add end-to-end exec tests for remote shell syntax, streamed output, and piped input through the in-process server in `tests/integration/ssh_server_test.go`

### Implementation for User Story 1

- [X] T012 [US1] Implement the command application service in `internal/app/command.go`, including selector resolution, revision pinning, host-verification gate reuse, non-interactive credential retrieval, and safe result normalization
- [X] T013 [US1] Implement non-PTY SSH command execution and close-on-cancellation lifecycle in `internal/sshclient/client.go` and `internal/sshclient/session.go`
- [X] T014 [US1] Register `orza exec` with one command argument after `--`, positive configurable `--timeout`, direct process streams, and `--json` rejection in `internal/cli/exec.go` and `internal/cli/root.go`
- [X] T015 [US1] Construct the command service with the existing catalog, credential, host-trust, and SSH adapters in `cmd/orza/main.go`
- [X] T016 [US1] Make the P1 tests pass and verify the scripted scenarios in `specs/014-cli-command-execution/quickstart.md`

**Checkpoint**: P1 is independently usable from a script or agent and does not open or require a terminal interface.

---

## Phase 4: User Story 2 - Receive Safe, Actionable Automation Failures (Priority: P2)

**Goal**: Automation fails promptly and safely for host-trust, credential, connection, timeout, cancellation, and interruption cases without secret exposure or resource leaks.

**Independent Test**: Invoke `exec` without a terminal against unknown and changed hosts, unavailable credentials, a missing or stale connection, an unreachable endpoint, and a timed-out command; verify no command runs where prohibited, no prompt or secret appears, the documented local status is returned, and resources are released.

### Tests for User Story 2

- [X] T017 [P] [US2] Add command-service failure tests for unknown/changed host rejection before secret access, unavailable non-interactive credentials, and stale revision rejection in `internal/app/command_test.go`
- [X] T018 [P] [US2] Add SSH command lifecycle tests for deadline expiry, context cancellation, transport interruption, close-once cleanup, and no retained large output in `internal/sshclient/session_test.go`
- [X] T019 [P] [US2] Add integration security tests for no host-trust persistence, no terminal secret prompt, timeout cleanup, and separated high-volume streams in `tests/integration/security_test.go`
- [X] T020 [P] [US2] Add CLI error-mapping tests for safe contextual diagnostics and documented usage, not-found, conflict, security, canceled, and transport statuses in `internal/cli/exec_test.go`

### Implementation for User Story 2

- [X] T021 [US2] Classify command startup, authentication, host-trust, cancellation, and deadline failures without exposing secrets in `internal/app/ssh_start_failure.go` and `internal/app/command.go`
- [X] T022 [US2] Map command failures to existing local CLI statuses while preserving completed remote statuses unchanged in `internal/cli/exec.go` and `internal/cli/exit.go`
- [X] T023 [US2] Make the P2 failure and cleanup tests pass and verify all error scenarios in `specs/014-cli-command-execution/quickstart.md`

**Checkpoint**: P1 and P2 both work independently, and every required non-interactive failure path is safe and observable.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Document the public command, verify regressions, and complete repository quality gates.

- [X] T024 [P] Document `orza exec`, its non-interactive trust behavior, `--timeout`, stream semantics, JSON exclusion, and exit-status caveat in `README.md`
- [X] T025 [P] Reconcile implemented command behavior with `specs/014-cli-command-execution/contracts/exec.md` and `specs/014-cli-command-execution/quickstart.md`
- [X] T026 Run formatting, static analysis, the full automated suite, and the quickstart regression checks referenced by `README.md` and `specs/014-cli-command-execution/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: Starts immediately. T001 and T002 can run in parallel.
- **Foundational (Phase 2)**: T003 and T004 can run in parallel; T005 follows T003; T006 follows T003 and T005; T007 follows T004 and T006. It blocks both stories.
- **User Story 1 (Phase 3)**: Starts after T007. Tests T008-T011 can run in parallel, then T012-T015 proceed in order, followed by T016.
- **User Story 2 (Phase 4)**: Starts after the foundational phase; it may be developed in parallel with P1 after shared SSH command behavior is available, but sequential P1 then P2 is recommended for the smallest reviewable delivery.
- **Polish (Phase 5)**: Starts after the desired stories are complete.

### User Story Dependencies

- **US1 (P1)**: Depends only on the foundational phase and is the MVP.
- **US2 (P2)**: Depends on the foundational phase and the shared command service/runner introduced for US1; it adds failure-path coverage and error presentation without changing the P1 command contract.

### Parallel Opportunities

- T001 and T002 prepare different test files in parallel.
- T003 and T004 touch different application files in parallel.
- T008-T011 are independent P1 test files and can run in parallel.
- T017-T020 are independent P2 test files and can run in parallel.
- T024 and T025 can run in parallel after behavior is stable.

## Parallel Example: User Story 1

```text
Task: "Add CLI exec contract tests in internal/cli/exec_test.go"
Task: "Add command-service tests in internal/app/command_test.go"
Task: "Add non-PTY SSH runner tests in internal/sshclient/session_test.go"
Task: "Add end-to-end exec tests in tests/integration/ssh_server_test.go"
```

## Parallel Example: User Story 2

```text
Task: "Add command-service failure tests in internal/app/command_test.go"
Task: "Add SSH lifecycle failure tests in internal/sshclient/session_test.go"
Task: "Add integration security tests in tests/integration/security_test.go"
Task: "Add CLI error-mapping tests in internal/cli/exec_test.go"
```

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete setup and foundational tasks through T007.
2. Complete P1 tests and implementation through T016.
3. Run the P1 independent test criteria with a non-interactive process and in-process SSH server.
4. Review the command contract before adding failure-path hardening.

### Incremental Delivery

1. Deliver US1 for normal trusted, non-interactive command execution.
2. Deliver US2 for host-trust, credential, cancellation, timeout, and cleanup failures.
3. Complete documentation and full regression validation.
