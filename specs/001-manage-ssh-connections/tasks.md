---

description: "Implementation tasks for cross-platform SSH connection management"
---

# Tasks: Gestión de conexiones SSH

**Input**: Design documents from `/specs/001-manage-ssh-connections/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/`, `quickstart.md`

**Tests**: Automated tests are required by the project constitution. Within each phase, write the
listed tests first and confirm that they fail for the expected reason before implementing behavior.

**Organization**: Tasks are grouped by user story so each increment has an independent acceptance
path. CLI and TUI use the same application services; later stories extend, rather than duplicate,
earlier behavior.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel after its documented prerequisites because it changes different files.
- **[Story]**: Maps the task to a user story from `spec.md`.
- Every task names its implementation or validation file path.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Initialize the Go project and reproducible Nix workflow.

- [X] T001 Initialize the Go 1.26 module as `github.com/pluque01/orza`, pin planned dependencies, and create the package directories in `go.mod`, `go.sum`, `cmd/orza/`, `internal/`, and `tests/integration/`
- [X] T002 [P] Define the pinned development shell, quality checks, native packages, and Windows cross-build outputs in `flake.nix` and `flake.lock`
- [X] T003 [P] Add the `realMain` entry point, version variables, and deferred-cleanup-safe exit boundary in `cmd/orza/main.go`
- [X] T004 [P] Ignore Go, Nix, editor, test, and local catalog artifacts without ignoring specifications in `.gitignore`

**Checkpoint**: `nix develop`, `go test ./...`, and a minimal `go build ./cmd/orza` can start from a clean checkout.

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Establish shared domain, persistence, platform security, application ports, and process
wiring before implementing a user-facing story.

**CRITICAL**: No user story implementation starts until this phase passes.

### Foundation Tests

- [X] T005 [P] Add failing table and fuzz tests for IDs, names, logical paths, node kinds, and revisions in `internal/domain/id_test.go`, `internal/domain/name_test.go`, and `internal/domain/path_test.go`
- [X] T006 [P] Add failing tests for catalog creation, PRAGMAs, schema identity, migration rollback, newer-schema rejection, and hot-journal recovery in `internal/catalog/store_test.go` and `internal/catalog/migrations_test.go`
- [X] T007 [P] Add failing platform tests for local data paths, symlink rejection, POSIX modes, and Windows DACL validation in `internal/catalog/location_unix_test.go` and `internal/catalog/location_windows_test.go`

### Foundation Implementation

- [X] T008 Implement immutable IDs, node kinds, revisions, validation errors, and safe error context in `internal/domain/id.go`, `internal/domain/node.go`, and `internal/domain/errors.go`
- [X] T009 Implement the absolute logical path grammar, normalization, and segment validation in `internal/domain/path.go`
- [X] T010 [P] Add the initial SQLite schema for catalog metadata, nodes, connection details, trusted hosts, credential operations, indexes, and constraints in `internal/catalog/migrations/001_initial.sql` and `internal/catalog/migrations.go`
- [X] T011 Implement catalog open/close, required PRAGMAs, schema checks, migration transactions, integrity checks, and one-connection pooling in `internal/catalog/store.go`
- [X] T012 [P] Implement XDG/macOS local data directories, owner checks, `0700` directory and `0600` database enforcement, and symlink rejection in `internal/catalog/location_unix.go`
- [X] T013 [P] Implement `%LOCALAPPDATA%`, protected current-user DACL creation, inheritance, and validation in `internal/catalog/location_windows.go`
- [X] T014 Define application repository, credential, host-trust, terminal, clock, and session ports plus request/result types in `internal/app/ports.go` and `internal/app/types.go`
- [X] T015 [P] Define the cancellable `CredentialStore` contract and fault-injectable in-memory fake without global state in `internal/credential/store.go` and `internal/credential/fake.go`
- [X] T016 [P] Define terminal state, size, raw-mode, resize, and secret-prompt contracts with a deterministic fake in `internal/terminal/terminal.go` and `internal/terminal/fake.go`
- [X] T017 Implement Cobra root construction, global flags, JSON/human response envelopes, redacted errors, and management exit codes in `internal/cli/root.go`, `internal/cli/output.go`, and `internal/cli/exit.go`
- [X] T018 Implement dependency bootstrap, catalog lifecycle, startup integrity failure handling, and a recovery hook for pending credential operations in `internal/app/bootstrap.go`
- [X] T019 Wire `realMain` to bootstrap and the Cobra root while ensuring all cleanup completes before `os.Exit` in `cmd/orza/main.go`

**Checkpoint**: Foundation tests pass; an empty private catalog opens on each platform and the command
root returns deterministic help, JSON, and safe errors.

---

## Phase 3: User Story 1 - Gestionar e iniciar conexiones por comandos (Priority: P1) MVP

**Goal**: Create, read, update, move, delete, and connect to saved SSH destinations through CLI
commands, including secure authentication, host verification, conflict rejection, and credential
lifecycle.

**Independent Test**: Using only CLI commands, create a connection, inspect and update it, reject a
stale update, establish a controlled SSH session through each authentication method, verify terminal
restoration, and delete the connection and any remembered password.

### Tests for User Story 1

- [X] T020 [P] [US1] Add failing repository tests for connection CRUD, validation, stable IDs, case-sensitive names, revisions, moves, and same-item conflicts in `internal/catalog/connection_repository_test.go`
- [X] T021 [P] [US1] Add failing CLI contract tests for connection commands, flags, confirmations, JSON envelopes, redaction, and exit statuses in `internal/cli/connection_commands_test.go` and `internal/cli/connect_command_test.go`
- [X] T022 [P] [US1] Add failing saga tests for save, replace, remove, delete, compensation failures, process-recovery phases, and orphan prevention in `internal/app/credential_saga_test.go`
- [X] T023 [P] [US1] Add failing host-key tests for known, unknown, changed, revoked, trust-once, persisted trust, and concurrent replacement paths in `internal/hostkey/policy_test.go`
- [X] T024 [P] [US1] Add failing SSH lifecycle tests for dial cancellation, auth failures, PTY setup, resize, streaming, remote statuses, cleanup order, and terminal restoration in `internal/sshclient/client_test.go` and `internal/sshclient/session_test.go`
- [X] T025 [P] [US1] Add a failing end-to-end CLI test with a controlled SSH server and fake credential/terminal boundaries in `tests/integration/cli_connection_test.go` and `tests/integration/ssh_server_test.go`

### Implementation for User Story 1

- [X] T026 [P] [US1] Implement connection details, authentication methods, field combinations, ports, and non-secret projections in `internal/domain/connection.go`
- [X] T027 [US1] Implement transactional connection create/read/list/update/move/delete with path resolution and revision compare-and-swap in `internal/catalogrepo/connection_repository.go`
- [X] T028 [US1] Implement durable credential save/replace/remove/delete sagas, verification, compensation, and startup recovery in `internal/app/credential_saga.go` and `internal/catalog/credential_operations.go`
- [X] T029 [P] [US1] Implement Windows Credential Manager set/get/delete with explicit local persistence and post-operation verification in `internal/credential/store_windows.go`
- [X] T030 [P] [US1] Implement non-synchronizing macOS Keychain set/get/delete with prompt cancellation and post-operation verification in `internal/credential/store_darwin.go`
- [X] T031 [P] [US1] Implement Linux Secret Service set/get/delete with bounded D-Bus prompts, dismissal handling, and no fallback in `internal/credential/store_linux.go`
- [X] T032 [P] [US1] Implement standard `known_hosts` read-only verification, revocation handling, application trust records, fingerprints, and explicit trust policy in `internal/hostkey/policy.go` and `internal/catalog/trusted_host_repository.go`
- [X] T033 [P] [US1] Implement Unix SSH-agent discovery and lifecycle through `SSH_AUTH_SOCK` in `internal/sshclient/agent_unix.go`
- [X] T034 [P] [US1] Implement Windows OpenSSH-agent named-pipe discovery and lifecycle through `go-winio` in `internal/sshclient/agent_windows.go`
- [X] T035 [US1] Implement agent, private-key/passphrase, and password auth providers with bounded retries and secret clearing where possible in `internal/sshclient/auth.go`
- [X] T036 [P] [US1] Implement POSIX terminal state, raw mode, size, resize signals, secret input, and guaranteed restoration in `internal/terminal/terminal_unix.go`
- [X] T037 [P] [US1] Implement Windows console state, raw mode, size polling, secret input, and guaranteed restoration in `internal/terminal/terminal_windows.go`
- [X] T038 [US1] Implement context-aware TCP dialing, SSH handshake, pre-auth host verification, resource ownership, and structured connection outcomes in `internal/sshclient/client.go`
- [X] T039 [US1] Implement PTY request, shell start, bounded I/O streaming, resize propagation, cancellation, cleanup, and remote exit mapping in `internal/sshclient/session.go`
- [X] T040 [US1] Implement shared connection CRUD use cases, confirmation scopes, expected revisions, and safe error mapping in `internal/app/connections.go`
- [X] T041 [US1] Implement the connect use case, trust decisions, secret acquisition, session ownership, and pre-session recovery behavior in `internal/app/connect.go`
- [X] T042 [P] [US1] Implement CLI create/list/show commands and stable human/JSON projections in `internal/cli/connection_create.go` and `internal/cli/connection_read.go`
- [X] T043 [P] [US1] Implement CLI update/move/delete commands, mutually exclusive flags, stale revision handling, and destructive confirmation in `internal/cli/connection_mutate.go`
- [X] T044 [US1] Implement the CLI connect command, terminal-only secret/trust prompts, no-JSON guard, and remote/local status propagation in `internal/cli/connect.go`
- [X] T045 [US1] Register concrete catalog, credential, host-key, agent, terminal, SSH, and CLI adapters in `cmd/orza/main.go`, `internal/app/bootstrap.go`, and `internal/cli/root.go`

**Checkpoint**: User Story 1 passes independently through `tests/integration/cli_connection_test.go` and
delivers the command-line MVP without requiring the TUI or nested-folder management.

---

## Phase 4: User Story 2 - Gestionar conexiones de forma interactiva (Priority: P2)

**Goal**: Expose connection CRUD and SSH initiation through a keyboard-only Bubble Tea interface with
recoverable errors, safe confirmations, resize support, and no-color usability.

**Independent Test**: Seed a root-level connection through the application fixture, then use only TUI
messages and keyboard input to create, inspect, edit, connect, cancel, delete, recover from a stale
write, and exit with the terminal restored.

### Tests for User Story 2

- [X] T046 [P] [US2] Add failing model and golden-view tests for empty state, navigation, help, 80x24 boundary, resize, narrow fallback, and `NO_COLOR` in `internal/tui/model_test.go` and `internal/tui/testdata/`
- [X] T047 [P] [US2] Add failing tests for connection forms, focus order, validation, discard prompts, destructive confirmation, trust prompts, secret consent, and stale conflicts in `internal/tui/connection_form_test.go` and `internal/tui/modal_test.go`
- [X] T048 [P] [US2] Add a failing TUI integration test for CRUD, pre-session failure recovery, session handoff, and terminal restoration in `tests/integration/tui_connection_test.go`

### Implementation for User Story 2

- [X] T049 [P] [US2] Define the global keymap, contextual help, ASCII status semantics, `NO_COLOR`, and terminal-aware styles in `internal/tui/keys.go` and `internal/tui/styles.go`
- [X] T050 [US2] Implement the Bubble Tea root model, catalog browser, selection, details, empty state, revision refresh, resize, and narrow-terminal fallback in `internal/tui/model.go` and `internal/tui/browser.go`
- [X] T051 [P] [US2] Implement connection create/edit forms, field dependencies, keyboard focus, validation, and dirty-state cancellation in `internal/tui/connection_form.go`
- [X] T052 [P] [US2] Implement destructive confirmations, stale-conflict recovery, actionable error modals, loading state, and late-message suppression in `internal/tui/modal.go` and `internal/tui/errors.go`
- [X] T053 [P] [US2] Implement host-trust and no-echo secret prompts with explicit password-storage consent in `internal/tui/trust_prompt.go` and `internal/tui/secret_prompt.go`
- [X] T054 [US2] Implement Bubble Tea shutdown/restart around SSH, pre-active failure return, active-session handoff, and final status propagation in `internal/tui/session.go`
- [X] T055 [US2] Launch the TUI only for no-command interactive invocation and wire shared use cases without CLI coupling in `internal/cli/root.go` and `internal/tui/run.go`

**Checkpoint**: User Story 2 passes independently through `tests/integration/tui_connection_test.go`; all
connection tasks are possible without memorizing commands.

---

## Phase 5: User Story 3 - Organizar conexiones en carpetas anidadas (Priority: P3)

**Goal**: Add nested-folder CRUD, moving, cycle prevention, recursive deletion, and equivalent CLI/TUI
navigation while preserving connection IDs and credential safety.

**Independent Test**: Create `/clientes/acme/produccion`, move connections and folders through CLI and
TUI, reject a cycle and a stale recursive confirmation, then delete the subtree without affecting
unrelated nodes or leaving credentials.

### Tests for User Story 3

- [X] T056 [P] [US3] Add failing repository tests for folder CRUD, shared namespace, root invariants, recursive path resolution, cycle rejection, subtree snapshots, and stale recursive deletion in `internal/catalogrepo/folder_repository_test.go`
- [X] T057 [P] [US3] Add failing CLI contract tests for folder commands, recursive scopes, confirmations, JSON, conflicts, and root protection in `internal/cli/folder_commands_test.go`
- [X] T058 [P] [US3] Add failing TUI tests for breadcrumbs, nested navigation, folder forms, move picker exclusions, subtree counts, and changed-scope cancellation in `internal/tui/folder_test.go`
- [X] T059 [P] [US3] Add a failing end-to-end hierarchy test covering equivalent CLI and TUI results and credential cleanup in `tests/integration/folder_hierarchy_test.go`

### Implementation for User Story 3

- [X] T060 [US3] Implement transactional folder CRUD, recursive CTE traversal, cycle checks, subtree snapshots, and compare-and-swap deletion in `internal/catalogrepo/folder_repository.go`
- [X] T061 [US3] Implement folder create/read/list/rename/move use cases and shared namespace error mapping in `internal/app/folders.go`
- [X] T062 [US3] Implement recursive deletion orchestration across subtree revisions and per-connection credential sagas in `internal/app/folder_delete.go`
- [X] T063 [P] [US3] Implement CLI folder create/list/show/rename/move/delete commands and recursive confirmation summaries in `internal/cli/folder_commands.go`
- [X] T064 [P] [US3] Extend the TUI browser with breadcrumbs, folder-first ordering, nested navigation, and cross-folder connection moves in `internal/tui/browser.go`
- [X] T065 [US3] Implement folder forms, destination picker, disabled descendant targets, and recursive delete summaries in `internal/tui/folder_form.go` and `internal/tui/move_picker.go`
- [X] T066 [US3] Register folder repositories and use cases and complete CLI/TUI parity wiring in `cmd/orza/main.go`, `internal/app/bootstrap.go`, `internal/cli/root.go`, and `internal/tui/model.go`

**Checkpoint**: All three user stories pass independently; a hierarchy can be managed from either
interface and all persistence and credential invariants remain enforced.

---

## Phase 6: Polish & Cross-Cutting Concerns

**Purpose**: Verify security, performance, native integrations, release builds, and user documentation
across all stories.

- [X] T067 [P] Add fuzz tests for paths, names, command arguments, host input, schema decoding, and state transitions in `internal/domain/path_fuzz_test.go`, `internal/cli/input_fuzz_test.go`, and `internal/catalog/schema_fuzz_test.go`
- [X] T068 [P] Add security regression tests for redaction, unsafe host arguments, private-key modes, trust-before-auth, bounded session output, and unchanged `~/.ssh/config` in `tests/integration/security_test.go`
- [X] T069 [P] Add benchmarks and scale fixtures for 1,000 connections, 100 folders, 10 levels, path lookup, list rendering, and recursive validation in `internal/catalog/benchmark_test.go` and `internal/tui/benchmark_test.go`
- [X] T070 [P] Add Windows-native integration tests for Credential Manager, OpenSSH-agent named pipes, DACLs, console restore, and the Windows release binary in `tests/integration/platform_windows_test.go`
- [X] T071 [P] Add macOS-native integration tests for non-synchronizing Keychain, file modes, terminal restore, and Intel/Apple Silicon release binaries in `tests/integration/platform_darwin_test.go`
- [X] T072 [P] Add Linux-native integration tests for Secret Service available/locked/headless states, agent sockets, POSIX modes, and release binaries in `tests/integration/platform_linux_test.go`
- [X] T073 Finalize Nix `vendorHash`, format/vet/staticcheck/govulncheck/race checks, six release outputs, deterministic version flags, and native smoke jobs in `flake.nix` and `flake.lock`
- [X] T074 [P] Document install, platform matrix, 80x24 requirement, CLI/TUI usage, SSH compatibility limits, credential stores, recovery, and security warnings in `README.md`
- [ ] T075 Execute every scenario in `specs/001-manage-ssh-connections/quickstart.md` on disposable data and record platform results and any manual SSH-boundary evidence in `specs/001-manage-ssh-connections/quickstart-results.md`
- [ ] T076 Run the complete Nix and native quality gates, resolve failures without weakening tests, and record final constitution compliance in `specs/001-manage-ssh-connections/validation.md`

**Checkpoint**: The release matrix builds reproducibly, native boundaries pass on all three systems,
the quickstart succeeds, and no constitutional exception remains.

---

## Dependencies & Execution Order

### Phase Dependencies

```text
Phase 1 Setup
  -> Phase 2 Foundation
      -> Phase 3 US1 CLI MVP
          -> Phase 4 US2 TUI
              -> Phase 5 US3 Nested folders
                  -> Phase 6 Polish and release validation
```

- Phase 1 has no dependencies.
- Phase 2 depends on module and toolchain setup and blocks every story.
- US1 depends only on Phase 2 and delivers the MVP.
- US2 depends on US1 application services but is independently testable through the TUI without CLI
  interaction.
- US3 repository and use-case work may begin after Phase 2, but the complete story depends on US1 and
  US2 adapters to prove CLI/TUI parity.
- Phase 6 depends on all stories selected for release.

### User Story Dependency Graph

```text
US1 (P1, CLI CRUD + connect)
  -> US2 (P2, interactive connection management)
      -> US3 (P3, nested organization in both adapters)
```

### Within Each User Story

1. Write the story's tests and verify expected failures.
2. Implement or extend domain and persistence behavior.
3. Implement application use cases and external security boundaries.
4. Implement CLI or TUI adapters.
5. Wire adapters and pass the independent integration test.

## Parallel Opportunities

- Setup tasks T002-T004 can run concurrently.
- Foundation tests T005-T007 can run concurrently; platform paths T012-T013 and ports T015-T016 can
  be split by file and platform.
- All six US1 test tasks can be authored concurrently. Credential adapters T029-T031, agent adapters
  T033-T034, and terminal adapters T036-T037 are independent platform streams.
- US2 tests T046-T048 can run concurrently; forms and modal/prompt components T051-T053 can proceed in
  parallel after the root model contract is fixed.
- US3 tests T056-T059 can run concurrently; CLI T063 and browser T064 can proceed in parallel after
  folder use cases are available.
- Cross-cutting tests T067-T072 and documentation T074 can run in parallel before final Nix and
  quickstart validation.

## Parallel Example: User Story 1

```text
Task T020: connection repository tests in internal/catalog/connection_repository_test.go
Task T021: CLI contract tests in internal/cli/connection_commands_test.go
Task T022: credential saga tests in internal/app/credential_saga_test.go
Task T023: host trust tests in internal/hostkey/policy_test.go
Task T024: SSH lifecycle tests in internal/sshclient/client_test.go
Task T025: CLI integration fixture in tests/integration/cli_connection_test.go
```

After interfaces stabilize:

```text
Task T029: Windows Credential Manager adapter
Task T030: macOS Keychain adapter
Task T031: Linux Secret Service adapter
Task T033: Unix agent adapter
Task T034: Windows agent adapter
Task T036: POSIX terminal adapter
Task T037: Windows terminal adapter
```

## Parallel Example: User Story 2

```text
Task T046: model and golden tests
Task T047: forms and modal tests
Task T048: TUI integration test
```

After `internal/tui/model.go` establishes message contracts:

```text
Task T051: connection form
Task T052: confirmations and errors
Task T053: trust and secret prompts
```

## Parallel Example: User Story 3

```text
Task T056: folder repository tests
Task T057: folder CLI tests
Task T058: folder TUI tests
Task T059: hierarchy integration test
```

After folder use cases are complete:

```text
Task T063: folder CLI commands
Task T064: nested TUI browser
```

## Implementation Strategy

### MVP First: User Story 1

1. Complete Setup and Foundation.
2. Implement US1 tests first, then connection persistence, credential/host security, SSH lifecycle,
   shared use cases, and CLI commands.
3. Stop after T045 and validate `tests/integration/cli_connection_test.go` on all three systems.
4. Release or demonstrate the CLI MVP before adding TUI complexity.

### Incremental Delivery

1. **Foundation**: Private transactional catalog and safe process boundaries.
2. **US1**: Scriptable connection CRUD and secure SSH session.
3. **US2**: Keyboard-first interactive workflow using the same application services.
4. **US3**: Nested organization and recursive operations in both adapters.
5. **Polish**: Native matrix, scale, security, documentation, and reproducible release artifacts.

### Team Strategy

After Foundation, one stream can own US1 application/repository work while platform specialists build
credential, agent, and terminal adapters marked `[P]`. During US2, view components can split after the
root message model is agreed. During US3, repository/use-case work precedes parallel CLI and TUI
adapter updates.

## Notes

- `[P]` never means tests may be skipped or implementation may start before its expected failing test.
- Platform-specific tasks require native validation even when a cross-build succeeds.
- Never add secret values to fixtures, command lines, environment variables, logs, or golden files.
- Keep all user-facing behavior aligned with `contracts/cli.md` and `contracts/tui.md`.
- Stop at each checkpoint to preserve an independently demonstrable increment.
