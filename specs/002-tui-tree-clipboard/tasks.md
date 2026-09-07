---

description: "Task list for contextual TUI tree and terminal-native paste"
---

# Tasks: Árbol contextual y pegado en TUI

**Input**: Design documents from `/specs/002-tui-tree-clipboard/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/tui.md`, `quickstart.md`

**Tests**: Input handling, terminal cleanup, reconciliation, and their principal failure paths require automated tests under the feature specification and constitution. Test tasks precede their corresponding implementation tasks.

**Organization**: Tasks are grouped by user story so the tree is an independently testable MVP and paste support is a separately testable increment.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it affects different files and has no dependency on another incomplete task in the same phase.
- **[Story]**: Maps the task to User Story 1 (`US1`) or User Story 2 (`US2`).
- Every task names the exact file or files it changes.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Make the already pinned terminal event parser an explicit reviewed dependency.

- [X] T001 [FR-012,FR-023] Promote `github.com/charmbracelet/ultraviolet` from indirect to direct without changing its pinned version in `go.mod`; retain `go.sum` unless module tooling changes checksums

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Confirm the existing project boundaries used by both stories.

No new foundational code is required. `internal/app` already exposes catalog IDs, revisions, hierarchy services, and optimistic mutation requests; `internal/terminal.Terminal` already owns secret input. User-story work starts after T001 without adding persistence or service abstractions.

**Checkpoint**: Existing application and terminal boundaries are ready for story work.

---

## Phase 3: User Story 1 - Navegar y actuar sobre el árbol (Priority: P1) MVP

**Goal**: Replace folder-scoped browsing with one keyboard-operated hierarchical tree whose identity-based selection determines every contextual action and survives catalog changes.

**Independent Test**: With three nested folder levels and several connections, expand/collapse and traverse the tree, perform each valid action from folder and connection selections, and observe identity-preserving selection or nearest-parent fallback after create, move, rename, delete, reload, and stale-target failures.

### Tests for User Story 1

> Write these tests first and ensure they fail for the missing tree behavior before implementation.

- [X] T002 [P] [US1] [FR-001–FR-005,FR-010–FR-011] Add unit tests for binary sibling ordering/ID tie-break, two-cell indentation, root initial/empty selection, ASCII markers, collapse fallback, expansion pruning, non-wrapping navigation, and nearest-parent fallback in `internal/tui/browser_test.go`
- [X] T003 [P] [US1] [FR-006,FR-009–FR-011,FR-020,FR-022,FR-025,SC-005] Add tests for revision retry, immutable connect confirmation, expected-revision rejection before network, pending selection, and form preservation in `internal/tui/model_test.go` and `internal/app/connect_test.go`
- [X] T004 [P] [US1] [FR-007–FR-008,FR-021,SC-001] Add exactly 20 executions of the three-level `n` scenario through successful save without destination correction, plus folder destinations and cancel state, in `internal/tui/connection_form_test.go` and `internal/tui/folder_test.go`
- [X] T005 [P] [US1] [FR-006–FR-011,FR-021–FR-022,SC-005] Add a complete create-folder/create-connection/rename/move/delete/connect matrix asserting captured target/result IDs and paths, concurrent connect rejection, root-invalid actions, and cancel behavior in `tests/integration/tui_tree_test.go`
- [X] T006 [P] [US1] [SC-004] Add `TestTreePerformanceAcceptance` plus `BenchmarkTreeRefresh`, `BenchmarkTreeExpand`, and `BenchmarkTreeCollapse`, each with internal warm-up and 20 timed reference-scale runs, in `internal/tui/benchmark_test.go`

### Implementation for User Story 1

- [X] T007 [US1] [FR-001–FR-004,FR-009–FR-011] Replace folder-scoped state with immutable snapshot indexes, binary ordered children, stable root identity, expansion IDs, selected ID, ancestor capture, and visible rows in `internal/tui/browser.go`
- [X] T008 [P] [US1] [FR-003,FR-019,FR-021–FR-022] Define non-conflicting tree navigation, expansion, contextual action, confirmation, reload, help, cancel, and exit bindings in `internal/tui/keys.go`
- [X] T009 [US1] [FR-006,FR-025] Implement asynchronous breadth-first loading with one catalog-revision retry, recoverable second-conflict handling, and atomic snapshot replacement in `internal/tui/model.go`
- [X] T010 [US1] [FR-003–FR-004,FR-019] Route non-wrapping movement, expand/collapse, parent/child traversal, root selection, help, and reload through the selected node in `internal/tui/model.go`
- [X] T011 [P] [US1] [FR-007–FR-008,FR-020–FR-021] Add explicit destination setters and contextual folder/connection initialization without persistence on cancel in `internal/tui/connection_form.go` and `internal/tui/folder_form.go`
- [X] T012 [US1] [FR-005–FR-008,FR-021–FR-022,SC-005] Bind captured targets, add optional `ConnectRequest.Expected`, compare it before runner/network, and confirm path/endpoint/revision in `internal/tui/model.go`, `internal/tui/modal.go`, `internal/tui/session.go`, `internal/app/types.go`, and `internal/app/connect.go`
- [X] T013 [US1] [FR-009–FR-011,FR-020] Reconcile selection and expansion after mutation, reload, conflict, and missing ancestors using pending IDs and captured ancestor chains in `internal/tui/model.go`
- [X] T014 [US1] [FR-006,FR-009,FR-021] Integrate selected sources and hierarchical destinations with cycle-safe move choices in `internal/tui/move_picker.go` and `internal/tui/model.go`
- [X] T015 [US1] [FR-001–FR-005,FR-019,SC-006] Render fixed ASCII markers, two-cell indentation, full-path details, and reduced mode preserving selected row/path/action/error/cancel at under 80x24 in `internal/tui/browser.go` and `internal/tui/styles.go`
- [X] T016 [US1] [SC-004] Run the acceptance test/benchmarks from `specs/002-tui-tree-clipboard/quickstart.md`; if fewer than 19 runs qualify, add the snapshot in `internal/app/folders.go`, `internal/catalogrepo/folder_repository.go`, `internal/tui/services.go`, and `internal/tui/model.go` with `internal/app/folders_test.go`, `internal/catalogrepo/folder_repository_test.go`, and `internal/tui/model_test.go`, then rerun

**Checkpoint**: User Story 1 is a functional, independently testable MVP with no paste changes required.

---

## Phase 4: User Story 2 - Pegar texto en cualquier entrada (Priority: P2)

**Goal**: Accept terminal-delivered paste in every normal and secret input as one atomic text operation, with cursor/selection replacement, safe control rejection, no shortcut dispatch, no OS clipboard reads, and deterministic terminal cleanup.

**Independent Test**: In every editable field, paste ASCII, Unicode, long values, shortcut letters, CR/LF, unsupported controls, empty payloads, and text over a partial selection; then repeat in secret prompts and establish that secrets remain masked and absent from output/errors/logs while terminal state is restored on success, cancel, context cancellation, and injected failures.

### Tests for User Story 2

> Write these tests first and ensure they fail for the missing paste behavior before implementation.

- [X] T017 [P] [US2] [FR-012–FR-016,FR-019] Add tests for cursor/Shift selection, non-color selection marker, five separators, controls, empty paste, 4,096/4,097-rune limits, Unicode viewport, and disabled OS clipboard in `internal/tui/text_field_test.go`
- [X] T018 [P] [US2] [FR-012–FR-016,SC-002] Apply every named ASCII/Unicode equivalence payload at start/middle/selection to all eight normal controls and compare manual/paste value plus validation in `internal/tui/connection_form_test.go` and `internal/tui/folder_test.go`
- [X] T019 [P] [US2] [FR-014,SC-003] Add root-model tests proving only the focused field receives `tea.PasteMsg` and the closed shortcut corpus never triggers navigation, mutation, confirmation, save, connect, cancel, or quit in `internal/tui/model_test.go`
- [X] T020 [P] [US2] [FR-013–FR-019,FR-024,SC-002,SC-007] Apply the named equivalence corpus within the byte limit and test Unicode selection, non-color masks, separators/controls, 4,096-byte limit, submit/cancel, wiping, and canary absence in `internal/terminal/secret_editor_test.go`
- [X] T021 [P] [US2] [FR-017–FR-018,FR-023–FR-024,SC-007–SC-008] Add the three-prompt × submit/cancel/I/O-failure matrix across every enumerated confidentiality surface, plus ownership, join, context/signals, cleanup, wiping, and exit 130/143 tests in `internal/terminal/secret_reader_test.go` and `cmd/orza/main_test.go`
- [X] T022 [P] [US2] [FR-023,SC-008] Extend Unix adapter tests for raw state, bracketed-paste lifecycle, newline, SIGINT/SIGTERM, restoration, and every `ReadSecret` return path in `internal/terminal/terminal_unix_test.go`
- [X] T023 [P] [US2] [FR-012,FR-018,FR-023,SC-008] Extend Windows tests for standard console, VT modes, cancellation, signal-equivalent context shutdown, CRLF, restoration, and unsupported handles in `internal/terminal/terminal_windows_test.go`
- [X] T024 [P] [US2] [FR-012–FR-018,FR-023,SC-002–SC-003,SC-007–SC-008] Add `//go:build !windows` PTY integration for all three prompt messages, manual/paste equivalence, controls, Shift selection, masking, signals, canary surfaces, and restoration in `tests/integration/terminal_paste_test.go`

### Implementation for User Story 2

- [X] T025 [US2] [FR-012–FR-016,FR-019] Implement the `textinput.Model` adapter with 4,096-rune limit, separator/control policy, non-color selection rendering, viewport, safe errors, and disabled Bubbles clipboard binding in `internal/tui/text_field.go`
- [X] T026 [P] [US2] [FR-012,FR-016,FR-020,SC-002] Migrate all seven connection metadata controls to the adapter while preserving focus, validation, dirty state, and contextual destination in `internal/tui/connection_form.go`
- [X] T027 [P] [US2] [FR-012,FR-016,FR-020,SC-002] Migrate folder `Name` to the adapter while preserving validation, dirty state, and cancel semantics in `internal/tui/folder_form.go`
- [X] T028 [US2] [FR-012,FR-014,FR-018,SC-003] Route `tea.PasteMsg` only to the focused field before global keys, ignore boundary messages, and expose the bracketed-paste limitation in help without OS clipboard access in `internal/tui/model.go`
- [X] T029 [P] [US2] [FR-013–FR-019,FR-024] Implement the private Unicode secret buffer with 4,096-byte limit, selection, separators/controls, non-color one-mask-per-rune rendering, safe status, and wiping in `internal/terminal/secret_editor.go`
- [X] T030 [US2] [FR-017–FR-018,FR-023–FR-024,SC-008] Implement reader cleanup plus root-owned `os.Interrupt`/SIGTERM context and exit 130/143 policy in `internal/terminal/secret_reader.go` and `cmd/orza/main.go`
- [X] T031 [P] [US2] [FR-023–FR-024,SC-008] Replace Unix `term.ReadPassword` with the dedicated reader while preserving `Terminal.ReadSecret`, signal cleanup, and deterministic restoration in `internal/terminal/terminal_unix.go`
- [X] T032 [P] [US2] [FR-012,FR-018,FR-023–FR-024,SC-008] Replace Windows `term.ReadPassword` with the dedicated standard-console reader and fail closed for non-cancelable alternate handles in `internal/terminal/terminal_windows.go`
- [X] T033 [US2] [SC-002–SC-003,SC-007–SC-008] Run and pass the closed-corpus TUI/terminal tests, Unix PTY integration, race detection, signal cleanup tests, and Windows cross-compilation from `specs/002-tui-tree-clipboard/quickstart.md`

**Checkpoint**: User Story 2 is independently testable across normal fields and the existing `Terminal.ReadSecret` boundary; US1 behavior remains unchanged.

---

## Phase 5: Polish & Cross-Cutting Concerns

**Purpose**: Validate performance, documentation, security, and native terminal behavior across both stories.

- [X] T034 [P] [FR-012,FR-019,FR-022–FR-024] Document tree controls, connection confirmation, limits, supported/degraded paste capabilities, and secret ownership/persistence/cleanup guarantees in `README.md`
- [X] T035 [SC-004] Record runner details, warm-up, timing boundaries, all 20 measurements per operation, and 19/20 qualification after T016 in `specs/002-tui-tree-clipboard/validation/performance.md`
- [X] T036 Run formatting, full tests, race, vet, staticcheck, govulncheck, `nix flake check`, and Windows cross-builds, recording commands/results and any justified environment exception in `specs/002-tui-tree-clipboard/validation/quality-gates.md`
- [ ] T037 [P] [FR-019,FR-023,SC-006–SC-008] Complete Linux amd64/arm64 validation with 20/20 `NO_COLOR` root→child→grandchild→root runs, exact 80x24/narrow modes, normal/secret selection, SIGINT/SIGTERM exit 130/143, and limits in `specs/002-tui-tree-clipboard/validation/linux.md`
- [ ] T038 [P] [FR-012,FR-019,FR-023,SC-002–SC-003,SC-006–SC-008] Complete Windows amd64/arm64 validation for the 20/20 keyboard sequence, three prompts, paste/modifiers, alternate-handle fail-closed, `os.Interrupt` exit 130, context cancellation, and restoration in `specs/002-tui-tree-clipboard/validation/windows.md`
- [ ] T039 [P] [FR-012,FR-019,FR-023,SC-002–SC-003,SC-006–SC-008] Complete macOS amd64/arm64 validation for the 20/20 keyboard sequence, three prompts, paste/modifiers, SIGINT/SIGTERM exit 130/143, cancellation, and restoration in `specs/002-tui-tree-clipboard/validation/macos.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: T001 has no dependency and can start immediately.
- **Foundational (Phase 2)**: Uses existing boundaries; begins after T001 and adds no tasks.
- **User Story 1 (Phase 3)**: Starts after Setup/Foundation and delivers the MVP.
- **User Story 2 (Phase 4)**: Logically testable through its own fields and terminal boundary, but follows US1 in the recommended sequence because both stories edit `model.go` and form files.
- **Polish (Phase 5)**: Starts after all stories selected for release are complete.

### User Story Dependencies

```text
T001 Setup
  └── US1 (P1, MVP)
        └── US2 (P2, recommended repository order)
              └── Polish and native validation
```

- **US1**: No dependency on US2; it can ship as the tree-navigation MVP.
- **US2**: Its `textField` and secret editor are independently testable, but final model/form integration assumes the US1 file layout.

### Within User Story 1

- T002-T006 can be authored in parallel and must fail before implementation.
- T007 and T008 can proceed in parallel after tests exist.
- T009 depends on T007; T010 depends on T008-T009.
- T011 can proceed in parallel with T007-T010; T012 depends on T010-T011.
- T013-T014 depend on T012; T015 depends on T007 and can proceed while action integration is completed.
- T016 runs after T002-T015 and owns any performance fallback before the US1 checkpoint.

### Within User Story 2

- T017-T024 can be authored in parallel and must fail before implementation.
- T025 follows T017; T026-T027 then proceed in parallel.
- T028 depends on T019 and T025-T027.
- T029 follows T020 and can proceed in parallel with T025-T028.
- T030 depends on T021 and T029; T031-T032 then proceed in parallel.
- T033 runs after T017-T032.

### Parallel Opportunities

- US1 test files T002-T006 are independent.
- US1 tree state/keymap tasks T007-T008 are independent.
- US2 test files T017-T024 are independent after US1 completes.
- Normal-input integration T026-T027 and platform adapters T031-T032 are parallel pairs.
- Documentation T034 and native validation T037-T039 can run in parallel because they write distinct files.

---

## Parallel Example: User Story 1

```text
Task T002: Add tree projection/navigation unit tests in internal/tui/browser_test.go
Task T003: Add async loading/action/reconciliation tests in internal/tui/model_test.go
Task T004: Add contextual destination tests in form test files
Task T005: Add SQLite-backed nested tree integration scenarios
Task T006: Add scale benchmarks in internal/tui/benchmark_test.go
```

## Parallel Example: User Story 2

```text
Task T017: Add normal text field paste/selection tests
Task T020: Add pure secret editor tests
Task T021: Add secret reader lifecycle tests
Task T022: Extend Unix terminal adapter tests
Task T023: Extend Windows console adapter tests
Task T024: Add Unix PTY integration coverage
```

---

## Implementation Strategy

### MVP First (User Story 1 Only)

1. Complete T001 and confirm existing foundational boundaries.
2. Write failing US1 tests T002-T006.
3. Implement T007-T015 in dependency order.
4. Run T016 and validate the tree independently.
5. Stop for MVP review before adding paste behavior.

### Incremental Delivery

1. Setup + existing foundation: dependency explicit and architecture unchanged.
2. US1: complete contextual tree, validate, and retain as a shippable MVP.
3. US2 normal inputs: complete T017-T019 and T025-T028, then validate independently.
4. US2 secret inputs: complete T020-T024 and T029-T032, then run T033.
5. Polish: documentation, scale gate, full quality gates, and native validation.

### Single-Developer Sequence

Follow numerical order. Do not run tasks marked `[P]` concurrently when another process is editing the same listed file. Commit only when explicitly requested.

---

## Notes

- `[P]` means different files and no dependency on another incomplete task in the same phase.
- Tests precede implementation and must demonstrate the missing behavior before the implementation task begins.
- Preserve unrelated worktree changes and never replace another node based on a stale row index.
- Never log or format raw paste events or secret buffers in tests, diagnostics, or errors.
- `checklists/readiness.md` is the requirements gate; implementation may proceed only while every item remains resolved and referenced.

---

## Phase 6: Convergence

- [ ] T040 Complete the 20/20 keyboard-only, `NO_COLOR`, 80x24/narrow, normal/secret selection, limit, SIGINT/SIGTERM exit, and restoration matrix in a real Linux amd64 terminal and finalize `specs/002-tui-tree-clipboard/validation/linux.md` per SC-006 and SC-008 (partial)
- [ ] T041 Execute the complete tree, three-prompt paste, modifier, signal, cancellation, cleanup, and restoration matrix on a native Linux arm64 host and record architecture/terminal evidence in `specs/002-tui-tree-clipboard/validation/linux.md` per SC-008 and plan: target platforms (missing)
- [ ] T042 Execute the complete 20/20 keyboard, three-prompt paste, modifier, alternate-handle fail-closed, `os.Interrupt` exit 130, cancellation, and restoration matrix on native Windows amd64 and arm64 consoles and finalize `specs/002-tui-tree-clipboard/validation/windows.md` per FR-012, FR-023, and SC-002–SC-003/SC-006–SC-008 (missing)
- [ ] T043 Execute the complete 20/20 keyboard, three-prompt paste, modifier, SIGINT/SIGTERM exit 130/143, cancellation, and restoration matrix on native macOS amd64 and arm64 terminals and finalize `specs/002-tui-tree-clipboard/validation/macos.md` per FR-012, FR-023, and SC-002–SC-003/SC-006–SC-008 (missing)
