---

description: "Executable tasks for responsive multipanel TUI redesign"
---

# Tasks: Rediseño multipanel de la TUI

> **Historical tasks**: Completed directional-marker tasks are superseded by `specs/005-add-scrollbar-indicator/tasks.md`; they must not be reintroduced.

**Input**: Design documents from `/specs/004-redesign-tui-layout/`

**Prerequisites**: `plan.md`, `spec.md`, `research.md`, `data-model.md`, `contracts/tui.md`, `quickstart.md`

**Tests**: Automated tests are mandatory because the specification and constitution require coverage for input routing, state ownership, resizing, terminal cleanup, host trust and failure paths. New behavior is red-first; preserved host-key decisions, paste, secret lifecycle, signals, exit codes and cleanup are baseline-passing characterization regressions.

**Organization**: Tasks are grouped by user story so every story can be implemented and tested as an incremental slice.

## Format: `[ID] [P?] [Story] Description`

- **[P]**: Can run in parallel because it changes a different file and has no dependency on another incomplete task in the same group.
- **[Story]**: Maps a task to its user story from `spec.md`.
- Every task names the exact file or files it changes.

## Phase 1: Setup (Shared Infrastructure)

**Purpose**: Establish a reproducible baseline before restructuring the existing TUI.

- [X] T001 Run `nix develop -c go test ./internal/tui`, `nix develop -c go test ./tests/integration -run 'TUI|FolderHierarchy|Terminal|HostKey'` and `nix develop -c go test ./...`, then record commands, results and environment limitations in `specs/004-redesign-tui-layout/quickstart.md`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: Introduce geometry, safe presentation, viewport, application focus, security input, generic modal, conflict and operation ownership shared by every story.

**Critical**: No user-story implementation starts until this phase passes its targeted tests.

### Foundational Tests

- [X] T002 [P] Add failing table tests for non-negative bounded wide, stacked, reduced and undersized rectangles plus centered overlays in `internal/tui/layout_test.go`
- [X] T003 [P] Add failing tests for exactly one application FocusOwner plus preemptive trust/secret SecurityInputOwner preservation across resize/results in `internal/tui/focus_test.go`
- [X] T004 [P] Add failing one-row, two-row and multi-row viewport budget tests for `↑ more`, `↓ more`, active-line priority and in-width `…` in `internal/tui/viewport_test.go`
- [X] T005 [P] Add failing safe-projection tests for ASCII, combining Unicode, CJK, emoji/ZWJ, invalid UTF-8, CR/LF, ANSI, C0/C1 and bidi controls in `internal/tui/safe_text_test.go`
- [X] T006 [P] Add failing operation lifecycle tests for one owner, monotonic IDs, commit points, cancellation, timeout, network interruption, quit, cleanup and stale/duplicate results in `internal/tui/operation_test.go`
- [X] T007 [P] Add failing conflict transition tests for missing/revised captured IDs, same-ID reload, ancestor Back, compact Cancel and operation-to-conflict cleanup precedence in `internal/tui/conflict_test.go`
- [X] T008 [P] Add failing generic modal-slot tests for typed payload registration, opener-focus restoration, background input isolation and no stack/replacement in `internal/tui/modal_test.go`
- [X] T009 [P] Run and extend baseline-passing characterization tests for bounded diagnostics, direct SSH streams, secret canaries, host-key decisions, timeout/network cleanup, signals, exit codes and terminal restoration in `internal/tui/security_regression_test.go`, `internal/tui/session_test.go`, `internal/sshclient/session_test.go`, `tests/integration/terminal_paste_test.go` and `tests/integration/ssh_failure_parity_test.go`

### Foundational Implementation

- [X] T010 [P] Implement geometry-only `LayoutState`, non-overlapping base rectangles and width-aware centered overlay rectangle calculation in `internal/tui/layout.go`
- [X] T011 [P] Implement active/inactive region, selected row, focused/invalid field, primary, muted, warning and failure styles with textual no-color semantics in `internal/tui/styles.go`
- [X] T012 [P] Implement bounded terminal-safe user/backend text projection without mutating source values or formatting raw causes in `internal/tui/safe_text.go`
- [X] T013 Implement reusable viewport offsets, active-line visibility, directional markers and ellipsis budgeting in `internal/tui/viewport.go`
- [X] T014 Add `FocusOwner`, `SecurityInputState`, `CapturedTarget`, `ConflictState` and `OperationState` with explicit invariant-preserving transitions to `internal/tui/model.go`
- [X] T015 Replace independent dialog/help flags with one generic typed `ModalState` slot, inline-Help state and opener restoration without implementing concrete workflow payloads in `internal/tui/modal.go` and `internal/tui/model.go`
- [X] T016 Compose the bounded Tree/Details/Actions shell and enforce `undersized > security input > operation/conflict > modal > form > Details > Tree` dispatch priority in `internal/tui/model.go`

**Checkpoint**: Shared state ownership, geometry and safe presentation are stable and all user stories may begin.

---

## Phase 3: User Story 1 - Explorar con contexto visible (Priority: P1) MVP

**Goal**: Keep the hierarchy visible while showing a current, non-secret detail projection for root, folder or connection and allowing read-only detail scrolling.

**Independent Test**: With nested folders and connections, move across root, connection, populated folder and empty folder; Details changes in the same transition, contains only direct folder connections, exposes no secret and scrolls without changing Tree selection.

### Tests for User Story 1

- [X] T017 [P] [US1] Add failing projection tests and a direct-child benchmark for connection fields, root/folder immediate children, stable ordering, empty folders and secret canaries in `internal/tui/detail_test.go`
- [X] T018 [P] [US1] Add failing Tree viewport tests for selected-row visibility, expansion preservation, long/deep Unicode rows and nearest-existing-ancestor fallback in `internal/tui/browser_test.go`
- [X] T019 [P] [US1] Add failing focus tests for same-transition Details refresh, `Tab`/`Shift+Tab`, read-only Details scrolling and target-change offset reset in `internal/tui/navigation_test.go`
- [X] T020 [P] [US1] Add failing catalog integration scenarios for connection detail, direct-only folder detail, explicit empty state and disappeared-selection fallback in `tests/integration/tui_tree_test.go`

### Implementation for User Story 1

- [X] T021 [P] [US1] Implement `DetailState`, safe connection/root/folder projection and shared viewport markers/ellipsis from snapshot immediate children in `internal/tui/detail.go`
- [X] T022 [P] [US1] Refactor browser rendering into the shared Tree viewport with active-row visibility, directional markers and safe ellipsis while preserving ordering, expansion and selected stable ID in `internal/tui/browser.go`
- [X] T023 [US1] Update Details and selection fallback atomically from stable IDs/revisions without row-index authorization in `internal/tui/model.go`
- [X] T024 [US1] Implement Tree/Details focus routing and read-only viewport controls without catalog mutation in `internal/tui/model.go` and `internal/tui/keys.go`
- [X] T025 [US1] Compose titled Tree and Details content with explicit focused, selected and empty-folder text semantics in `internal/tui/model.go` and `internal/tui/detail.go`

**Checkpoint**: US1 independently provides the browsable Tree plus accurate current Details and is the suggested MVP.

---

## Phase 4: User Story 2 - Descubrir y ejecutar acciones contextuales (Priority: P2)

**Goal**: Show only applicable actions for selection/interaction state and make Actions, Help and dispatch share one inventory, including single-owner asynchronous operation controls.

**Independent Test**: Select root, folder and connection, then enter normal and loading states; every displayed action is accepted, every unavailable key is inert, Help matches Actions, and cancellation/quit waits for owner cleanup without accepting stale results.

### Tests for User Story 2

- [X] T026 [P] [US2] Add failing exact `n/f/r/?/q`, `n/f/e/m/d/r/?/q` and `c/n/f/e/m/d/r/?/q` descriptor and compact-priority tables in `internal/tui/actions_test.go`
- [X] T027 [P] [US2] Add failing tests proving unlisted object keys are inert, `Ctrl+C` aliases Quit and connection `n/f` captures the parent in `internal/tui/action_dispatch_test.go`
- [X] T028 [P] [US2] Add failing four-kind operation matrices for status target, mutation suppression, Help-local scrolling, Cancel/Quit cleanup, pre/post-commit conflicts and stale results in `internal/tui/operation_integration_test.go`
- [X] T029 [P] [US2] Add failing integration scenarios for contextual inventories and captured destructive/connect targets in `tests/integration/tui_actions_test.go`
- [X] T030 [US2] Add failing 20-run SSH matrices for timeout/network interruption at every pre-active and active stage, 64 MiB stdout+stderr streaming, ≤1 MiB retained content, ≤256-character diagnostics and exactly-once cleanup in `internal/tui/session_test.go` and `internal/sshclient/session_test.go`

### Implementation for User Story 2

- [X] T031 [P] [US2] Implement canonical English action IDs, full/compact labels, priorities and applicability predicates in `internal/tui/actions.go`
- [X] T032 [US2] Align existing bindings and Help metadata with the descriptors implemented by T031 without changing established keys in `internal/tui/keys.go`
- [X] T033 [US2] Replace static footer and independent normal shortcut switches with descriptor-driven Actions rendering and dispatch in `internal/tui/model.go` and `internal/tui/actions.go`
- [X] T034 [US2] Implement exact `Loading: <action> — <target>` rendering, operation-only Cancel/Help/Quit inventory and shared Actions/Help viewport markers/ellipsis in `internal/tui/actions.go`
- [X] T035 [US2] Route initial load, reload, save and SSH start through one operation ID/owner and ignore extra mutations and non-owner results in `internal/tui/model.go`
- [X] T036 [US2] Apply pre/post-commit cancellation/timeout/network transitions, direct non-retained stdout/stderr streaming, confirmed-result reconciliation, exactly-once resource cleanup and terminal restore-before-return in `internal/tui/model.go`, `internal/tui/session.go` and `internal/sshclient/session.go`
- [X] T037 [US2] Derive normal/loading Help from current descriptors and allow only Help-local scroll/navigation beyond Cancel/Help/Quit during operations in `internal/tui/modal.go` and `internal/tui/actions.go`

**Checkpoint**: US2 independently exposes exact contextual controls and deterministic asynchronous ownership.

---

## Phase 5: User Story 3 - Editar conexiones en contexto (Priority: P3)

**Goal**: Render connection create/edit inside Details, preserve the visible Tree and form state, and implement safe Save/Discard/Cancel and conflict recovery.

**Independent Test**: Create and edit using only the keyboard, traverse all fields, resize with unsaved Unicode input, inject validation/persistence/conflict failures and exercise every dirty-Quit choice without value loss, secret exposure or retargeting.

### Tests for User Story 3

- [X] T038 [P] [US3] Add failing form tests for exact Name→Folder→Host→Port→User→Method→conditional Identity/Remember→Save order, reverse traversal, hidden-control skipping and focus return to Method when its change hides Identity/Remember in `internal/tui/connection_form_test.go`
- [X] T039 [P] [US3] Add failing Save/Discard/Cancel dirty-Quit matrices for success, validation error, persistence error, conflict and cleanup ordering in `internal/tui/dirty_exit_test.go`
- [X] T040 [P] [US3] Add failing form missing/revision-changed tests for captured ID, blocked Save, same-ID Reload, ancestor Back and compact Cancel in `internal/tui/form_conflict_test.go`
- [X] T041 [P] [US3] Add failing create/edit/cancel/save integration scenarios with Tree retained and post-save result selected in `tests/integration/tui_connection_test.go`
- [X] T042 [P] [US3] Add failing bracketed-paste, control-byte and secret-canary regressions through embedded-form focus, validation and resize in `internal/tui/embedded_form_security_test.go`

### Implementation for User Story 3

- [X] T043 [P] [US3] Implement canonical conditional focus order/skipping/Method fallback plus dynamic dimensions and shared viewport markers that keep the focused field, error and primary control visible in `internal/tui/connection_form.go` and `internal/tui/text_field.go`
- [X] T044 [P] [US3] Extend unsaved-changes payload with cancel-versus-quit intent and explicit Save/Discard/Cancel choices in `internal/tui/modal.go`
- [X] T045 [US3] Render connection create/edit inside Details, keep Tree visible/inert and restore captured selection/expansion on Cancel in `internal/tui/model.go`
- [X] T046 [US3] Preserve form values/focus/target with field-local validation errors and form-level inline save/persistence errors above Save without opening a modal in `internal/tui/model.go` and `internal/tui/errors.go`
- [X] T047 [US3] Implement blocked form conflict controls using captured ID/revision with no row-position retargeting in `internal/tui/model.go`
- [X] T048 [US3] Implement `quitAfterSave` so exit occurs only after save commit, snapshot reconciliation and operation cleanup in `internal/tui/model.go`
- [X] T049 [US3] Select the committed create/edit result on success while rejecting stale save results and preserving state on failure/conflict in `internal/tui/model.go`

**Checkpoint**: US3 independently supports safe contextual connection editing and deterministic dirty-form exit.

---

## Phase 6: User Story 4 - Resolver acciones en paneles centrados (Priority: P4)

**Goal**: Present exactly ten generic interactions in one centered panel while preserving preemptive trust/password/passphrase security input outside that panel.

**Independent Test**: Open all ten panel kinds and verify target/focus/recovery, then inject trust/password/passphrase boundaries and prove they preempt operation/application input, suspend below 40x12 and restore without secret exposure or implicit acceptance.

### Tests for User Story 4

- [X] T050 [P] [US4] Add failing centered-overlay tests for bounds, background preservation, Unicode width and reduced/undersized presentation in `internal/tui/modal_layout_test.go`
- [X] T051 [P] [US4] Add failing table tests for ten concrete modal kinds over one renderer, typed payload validation, visible cancel and rejection of unknown/nested/replacement kinds in `internal/tui/modal_inventory_test.go`
- [X] T052 [P] [US4] Add failing tests for embedded errors/conflicts, retained payload/target/focus and bounded inline Help toggle/scroll/Esc restoration without stack/replacement in `internal/tui/modal_recovery_test.go`
- [X] T053 [P] [US4] Add failing explicit-acceptance tests proving captured full path/endpoint/ID wrap without truncation and remain revealable for delete connection/folder and connect confirmation in `internal/tui/confirmation_test.go`
- [X] T054 [P] [US4] Add failing end-to-end folder create/edit, move, delete, Help and cancel-restoration scenarios in `tests/integration/folder_hierarchy_test.go`
- [X] T055 [P] [US4] Keep existing trust/secret lifecycle checks baseline-passing, then add failing trust/password/passphrase ownership matrices for precedence, masking, Cancel, normal/reduced resize, undersized suspension/recovery and terminal failure in `internal/tui/security_input_test.go`, `internal/tui/trust_prompt_test.go` and `internal/tui/secret_prompt_test.go`

### Implementation for User Story 4

- [X] T056 [P] [US4] Render modal content inside T010 geometry using shared viewport budgets, active error/recovery priority, directional markers and safe ellipsis in `internal/tui/modal.go`
- [X] T057 [P] [US4] Adapt folder create/edit fields, validation and captured destination to typed modal payloads in `internal/tui/folder_form.go`
- [X] T058 [P] [US4] Adapt move destination selection to the shared modal viewport with active-row visibility, directional markers and safe ellipsis while preserving descendant exclusions in `internal/tui/move_picker.go`
- [X] T059 [US4] Implement/register ten concrete modal payloads over the foundational slot with per-kind controls, bounded inline Help and opener restoration in `internal/tui/modal.go`
- [X] T060 [US4] Embed safe recoverable operation errors, SSH failures and conflicts in the current panel without losing payload or focus in `internal/tui/modal.go` and `internal/tui/errors.go`
- [X] T061 [US4] Route modal keys/results through the single owner and block every underlying Tree/Details/form input in `internal/tui/model.go`
- [X] T062 [US4] Require explicit captured-target acceptance for destructive and connection actions and make Cancel side-effect free in `internal/tui/model.go`
- [X] T063 [US4] Implement trust/secret SecurityInputOwner precedence, explicit unknown/changed decisions, password/passphrase masking, undersized suspension and cleanup outside ModalState in `internal/tui/session.go`, `internal/tui/trust_prompt.go`, `internal/tui/secret_prompt.go` and `internal/tui/run.go`

**Checkpoint**: US4 independently supplies the exact non-stacking panel inventory without weakening host trust.

---

## Phase 7: User Story 5 - Usar la interfaz en distintos tamaños (Priority: P5)

**Goal**: Apply deterministic wide/stacked orientation, orthogonal reduced priorities, undersized preservation and universal overflow to every state.

**Independent Test**: Repeat the fixed size, wide-short and 40x12→39x11→40x12 matrices with synthetic navigation, form, panel and operation payloads; geometry stays bounded and exact opaque state plus safe recovery controls survive every transition.

### Tests for User Story 5

- [X] T064 [P] [US5] Add failing exact geometry tables for `{40,60,79,80,100,160} × {12,24}` plus `39x12`/`40x11`, including rectangle formulas, border/padding/gutter and reduced priorities in `internal/tui/responsive_layout_test.go`
- [X] T065 [P] [US5] Add failing 20-run resize tests preserving selection, expansion, offsets, form values, focus, modal, conflict, operation and pending intent in `internal/tui/resize_state_test.go`
- [X] T066 [P] [US5] Add failing 40x12→39x11→40x12 tests asserting minimum/Help/safe Quit-only shell and exact restoration for navigation, dirty form, confirmation, operation and pending synthetic security input in `internal/tui/undersized_test.go`
- [X] T067 [P] [US5] Add failing universal overflow matrices for Tree, Details, form, Actions, Help, picker, confirmation and error payloads in `internal/tui/overflow_test.go`
- [X] T068 [P] [US5] Add failing exact no-color cue, closed keyboard map, unmodified F2 modifier fallback and safe Unicode/control-byte assertions across every display mode in `internal/tui/accessibility_test.go`
- [X] T069 [P] [US5] Add resize/focus/scroll/modal/conflict/operation/stale-result fuzz sequences and 10,000-cycle <1 MiB render-history retention test in `internal/tui/model_fuzz_test.go` and `internal/tui/performance_acceptance_test.go`
- [X] T070 [P] [US5] Add 20-sample monotonic selection/focus/scroll/resize/reload acceptance measurements on the 1,100-node fixture in `internal/tui/performance_acceptance_test.go`

### Implementation for User Story 5

- [X] T071 [US5] Implement undersized precedence, width-only wide/stacked orientation and orthogonal reduced status for width <80 or height <24 in `internal/tui/layout.go`
- [X] T072 [US5] Allocate complete 80x24 regions and reduced priorities that retain active/error/recovery/cancel/back/quit controls in `internal/tui/layout.go`
- [X] T073 [US5] Preserve opaque model state without clamping or mutation while undersized and reveal it exactly after recovery in `internal/tui/model.go`
- [X] T074 [US5] Restrict undersized dispatch before security/operation filtering to bounded Help, resize and safe Quit while preserving pending security input and dirty/operation cleanup in `internal/tui/model.go`
- [X] T075 [US5] Apply responsive rectangles and viewport budgets to generic synthetic Tree, Details, Actions, form and modal payload states in `internal/tui/layout.go` and `internal/tui/model.go`
- [X] T076 [US5] Render the bounded undersized notice with required `40x12`, Help and safe Quit while suppressing all base-region content in `internal/tui/model.go`
- [X] T077 [US5] Make `NO_COLOR` and `--no-color` remove ANSI while retaining every textual focus/status semantic in `internal/tui/styles.go` and `internal/cli/root.go`

**Checkpoint**: US5 independently proves responsive layout, undersized restoration, overflow and no-color behavior.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: Remove obsolete paths, prove English/safe output and collect every non-waivable quality artifact.

- [X] T078 [P] Document supported Linux/macOS/Windows VT capabilities, keyboard fallback, no mouse contract, multipanel layout, keys, reduced/undersized behavior, English/no-color and trust/password/passphrase ownership in `README.md`
- [X] T079 [P] Create the human-reviewed English manifest and extraction test that fails on set mismatch, extra/missing copy, duplicate canonical concepts, inconsistent terms or unclassified literals outside the user/backend allowlist in `internal/tui/testdata/controlled_english.txt` and `internal/tui/language_test.go`
- [X] T080 [P] Add integration regressions for terminal paste, bounded SSH diagnostics/streams, signals, timeout, network interruption, cancellation and restoration in `tests/integration/terminal_paste_test.go` and `tests/integration/ssh_failure_parity_test.go`
- [X] T081 Remove superseded full-screen/narrow rendering, static footer/help, modal flags and dead error paths from `internal/tui/browser.go`, `internal/tui/model.go`, `internal/tui/modal.go` and `internal/tui/errors.go`
- [x] T082 Run formatting, focused tests, `go test ./...`, race detector, vet, staticcheck and govulncheck, recording exact results or pending limitations in `specs/004-redesign-tui-layout/quickstart.md`
- [x] T083 Run `FuzzModelStateTransitions`, layout/detail/tree benchmarks and SC-011 performance acceptance, recording all 20 measurements in `specs/004-redesign-tui-layout/quickstart.md`
- [x] T084 Run `nix flake check --print-build-logs` and Linux/Windows/macOS amd64/arm64 pure-Go cross-builds, recording exact results in `specs/004-redesign-tui-layout/quickstart.md`
- [x] T085 Execute SC-001–SC-007 and SC-009–SC-017 exact geometry, every modal path, operation/conflict, security-input, stream/retained-memory, transport-cleanup, controlled-English and safe-text matrices and record evidence without pre-approving pending SC-008 in `specs/004-redesign-tui-layout/quickstart.md`
- [ ] T086 Perform native Linux, Windows and macOS terminal trust/password/passphrase checks where available and record ownership, masking, resize suspension, cleanup and pending hardware boundaries in `specs/004-redesign-tui-layout/quickstart.md`
- [ ] T087 Conduct the fixed 10-participant SC-008 study and record anonymous eligibility, timings, ratings and aggregate outcome in `specs/004-redesign-tui-layout/usability-study.md`
- [ ] T088 Aggregate T085 and SC-008 evidence, then complete constitution review for SSH trust, secrets, target stability, cleanup, keyboard recovery and mandatory-gate status in `specs/004-redesign-tui-layout/quickstart.md`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: No dependencies; captures the baseline.
- **Foundational (Phase 2)**: Depends on Setup and blocks all user stories.
- **US1-US3 and US5 (Phases 3-5, 7)**: Depend on Foundational; tests and primitives may proceed independently, while tasks changing `internal/tui/model.go` must be coordinated.
- **US4 (Phase 6)**: Modal/security mechanics start after Foundational, but exact ten-kind conformance depends on US3's unsaved-changes payload T044.
- **Polish (Phase 8)**: Depends on all selected stories; final gates require all five stories.

### User Story Dependency Graph

```text
Setup -> Foundational -> US1 (MVP) --\
                      -> US2 ---------+
                      -> US3 -------> US4 --+-> Polish
                      -> US5 ---------/
```

### User Story Dependencies

- **US1 (P1)**: No story dependency after Foundational; supplies independently useful Tree/Details navigation.
- **US2 (P2)**: No story dependency after Foundational; uses shared selection kinds and operation state.
- **US3 (P3)**: No story dependency after Foundational; embeds existing connection forms in the shared shell.
- **US4 (P4)**: Modal/security primitives may start after Foundational; independent phase completion and ten-kind conformance require US3 T044.
- **US5 (P5)**: No story dependency after Foundational for synthetic responsive mechanics; final concrete conformance in T085 depends on US1-US4.

### Within Each User Story

- Write new-behavior tests first and establish that they fail; run characterization regressions at baseline and require them to pass.
- Implement projection/state primitives before root-model integration.
- Preserve captured IDs/revisions and existing service/security semantics before presentation changes.
- Run targeted unit and integration tests before declaring the story checkpoint complete.
- Do not begin cross-cutting cleanup until all desired stories pass independently.

### Parallel Opportunities

- T002-T009 can run together; T010-T012 can then run together before T013-T016 integrate shared state.
- US1 tests T017-T020 and primitives T021-T022 are parallel groups.
- US2 tests T026-T029 are parallel; implement T031 before descriptor-dependent binding task T032 and root dispatch integration.
- US3 tests T038-T042 are parallel; T043-T044 are parallel before form/model integration.
- US4 new-behavior tests T050-T055 are parallel after baseline characterization cells in T055 pass; T056-T058 are parallel before modal/security integration.
- US5 tests T064-T070 are parallel before responsive implementation is integrated.
- T078-T080 are parallel; quality evidence T082-T088 is sequential after cleanup T081.

---

## Parallel Execution Examples

### User Story 1

```text
Task T017: Detail projection tests in internal/tui/detail_test.go
Task T018: Tree viewport tests in internal/tui/browser_test.go
Task T019: Navigation/focus tests in internal/tui/navigation_test.go
Task T020: Catalog integration tests in tests/integration/tui_tree_test.go
```

### User Story 2

```text
Task T026: Exact inventory tests in internal/tui/actions_test.go
Task T027: Dispatch-inertness tests in internal/tui/action_dispatch_test.go
Task T028: Operation matrix in internal/tui/operation_integration_test.go
Task T029: Contextual integration in tests/integration/tui_actions_test.go
```

### User Story 3

```text
Task T038: Form viewport tests in internal/tui/connection_form_test.go
Task T039: Dirty-exit tests in internal/tui/dirty_exit_test.go
Task T040: Form conflict tests in internal/tui/form_conflict_test.go
Task T041: Connection integration in tests/integration/tui_connection_test.go
Task T042: Embedded-form security in internal/tui/embedded_form_security_test.go
```

### User Story 4

```text
Task T050: Modal geometry in internal/tui/modal_layout_test.go
Task T051: Closed inventory in internal/tui/modal_inventory_test.go
Task T052: Modal recovery in internal/tui/modal_recovery_test.go
Task T053: Confirmation targets in internal/tui/confirmation_test.go
Task T054: Folder integration in tests/integration/folder_hierarchy_test.go
Task T055: Host trust in internal/tui/trust_prompt_test.go
```

### User Story 5

```text
Task T064: Exact dimensions in internal/tui/responsive_layout_test.go
Task T065: Resize preservation in internal/tui/resize_state_test.go
Task T066: Undersized transitions in internal/tui/undersized_test.go
Task T067: Overflow matrix in internal/tui/overflow_test.go
Task T068: Accessibility in internal/tui/accessibility_test.go
Task T069: State fuzzing in internal/tui/model_fuzz_test.go
Task T070: Performance acceptance in internal/tui/performance_acceptance_test.go
```

---

## Implementation Strategy

### MVP First (User Story 1)

1. Complete Setup and Foundational phases.
2. Complete US1 tests, safe detail projection, Tree viewport and focus integration.
3. Stop and run the independent test against connection, populated-folder, empty-folder and disappeared-selection cases.
4. Demonstrate target-stable Tree/Details navigation before adding mutations.

### Incremental Delivery

1. Foundation: geometry, safe text, viewport, ownership and panel shell.
2. US1: current Tree plus Details as the MVP.
3. US2: exact contextual Actions and asynchronous operation ownership.
4. US3: connection editing, conflict recovery and safe dirty exit.
5. US4: exact centered-panel inventory plus trust/password/passphrase input ownership.
6. US5: responsive/reduced/undersized rendering, overflow and no-color.
7. Polish: remove obsolete paths and collect mandatory evidence.

### Parallel Team Strategy

After Foundational completes, separate owners may implement tests and primitives for US1-US5 against the shared state contracts. Serialize or explicitly coordinate every task changing `internal/tui/model.go`; merge each story's primitives and tests before its model integration task. US5 uses synthetic payloads so its responsive contract does not require completed concrete workflows from US1-US4.

## Notes

- `[P]` tasks change independent files or stable shared contracts.
- Every user-story task carries its `[USn]` traceability label.
- New-behavior tests precede implementation and must fail; preserved-security characterization tests must pass before and after changes.
- No task adds Huh, a second Bubble Tea model, persistent UI state or new catalog/SSH semantics.
- A failed, unavailable or unrun mandatory gate remains pending and blocks merge; it is never waived or reported as passed.

## Phase 9: Convergence

- [X] T089 CRITICAL Gate live trust/password/passphrase input on current terminal dimensions, pause or cancel blocked readers on normal-to-undersized resize, resume without accepting buffered input, include remember-password ownership, and add concurrent runtime resize regressions per Constitution II/III and FR-005/FR-018/FR-023 (contradicts)
- [X] T090 CRITICAL Keep the Bubble Tea control path active during SSH start and remembered-password operations, or add an independently active joined control channel, so displayed Cancel/Help/Quit keys work during real execution and cleanup completes before restoration per Constitution II/III, FR-023 and SC-013 (contradicts)
- [X] T091 CRITICAL Apply terminal-safe projection before wrapping or truncating every dynamic trust, move-picker and form-conflict value, with ANSI, CR/LF, C0/C1, bidi, invalid UTF-8 and oversized regressions per Constitution Security and Operational Constraints, FR-024 and the plan threat model (contradicts)
- [X] T092 CRITICAL Make the connection-form Folder control target-stable by capturing and rechecking destination ID/path/revision and performing a pinned edit move, or make it consistently read-only and remove it from editable traversal, with destination-race tests per FR-012/FR-021 and the connection-form contract (contradicts)
- [X] T093 Suppress stale payload action lines while a modal operation is loading and render only captured content plus Cancel/Help/Quit controls per FR-010/FR-023 (partial)
- [X] T094 Remove the independent `Model.help`/Details replacement path, open `modalKindHelp` for standalone Tree/Details/form/operation Help, and retain inline Help only when another modal owns focus per FR-015/FR-016 and T081 (contradicts)
- [X] T095 Render bounded terminal-safe navigation notices when reload falls back from a missing node or refreshes a changed revision, and assert the first synchronized frame in normal, reduced and no-color modes per FR-021 (partial)
- [X] T096 Remove the product-level depth-10 rejection from catalog snapshot loading while preserving cycle, duplicate and consistency checks, or obtain an explicit approved product limit, per the plan no-domain-change decision (unrequested)
- [ ] T097 Make Linux Secret Service session, unlock, search, create, read and delete work context-aware or otherwise cancel and join pending D-Bus work safely, then record native prompt cancellation and dismissal evidence per T086 and Constitution III (partial)
- [ ] T098 Execute and record native Linux, Windows and macOS terminal checks for trust/password/passphrase ownership, masking, resize suspension, cancellation and restoration without waiving unavailable hardware per T086 and FR-027 (missing)
- [ ] T099 Conduct the fixed anonymous 10-participant study, record eligibility, exact-protocol timings and ratings, calculate the aggregate threshold, and create `specs/004-redesign-tui-layout/usability-study.md` per SC-008 and T087 (missing)
- [ ] T100 Aggregate completed automated, native and usability evidence and perform the final constitution review for trust, secrets, target stability, cleanup, keyboard recovery and mandatory-gate status per T088 and Constitution Governance (missing)
- [X] T101 Extend controlled-language extraction to all application-owned error copy that can reach TUI rendering, or map those errors to manifest-owned TUI copy, and require exact corpus equality per FR-024/SC-014 (partial)
- [X] T102 Add a testable required-VT capability boundary with actionable startup failure after terminal restoration, or narrow the documented guarantee to safely detectable capabilities, per FR-027 (partial)
- [X] T103 Align operation-error and SSH-failure visible key descriptors with the lowercase `d/r/e/b/q` contract, or bind and test every advertised uppercase variant, per FR-018 and `contracts/tui.md` (contradicts)
- [X] T104 Make `OperationState` the sole source of operation owner ID, phase, cancellation context and loading status, remove ownerless result acceptance and duplicated loading fields, and update stale-result tests per the plan operation-ownership decision (partial)
- [X] T105 Strengthen state fuzzing to mutate the actual Model modal/form-conflict/operation owners, execute generated commands and inject matching, stale and duplicate completions while asserting ownership and frame bounds per T069/T083 (partial)
- [X] T106 Correct Darwin packaging documentation to distinguish native cgo/Keychain flake outputs from pure-Go fallback or cross-build behavior and verify the native package boundary per T078/T084 (contradicts)
- [X] T107 Correct the README keyboard table to document `d` Discard and `Ctrl+C` safe Quit with the dirty Save/Discard/Cancel path per T078 and FR-018 (contradicts)
- [X] T108 Remove the duplicate shell-level `Host:` connection detail line and its manifest/test expectations, retaining the canonical `Endpoint` field per FR-001/FR-006 (unrequested)
