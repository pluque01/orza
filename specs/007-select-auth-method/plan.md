# Implementation Plan: Selección directa del método de autenticación

**Branch**: `007-select-auth-method` | **Date**: 2026-09-10 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `/specs/007-select-auth-method/spec.md`

**Note**: This template is filled in by the `/speckit.plan` command; its definition describes the execution workflow.

## Summary

Replace the editable authentication-method text field in create and edit connection forms with one closed, typed selector that renders `Agent`, `Key`, and `Password` simultaneously. Preserve the existing form field identity, focus traversal, cyclic Left/Right controls, dependent `Identity file` and `Remember password` state, viewport behavior, and create/update service boundaries. Ignore printable, editing, and paste input while the selector owns focus; persist only the selected method's applicable data and prevent a retained hidden remember choice from requesting or storing a password. Add no dependency, persistent format, protocol, or concurrency.

## Technical Context

**Language/Version**: Go 1.26 (`toolchain go1.26.6`)

**Primary Dependencies**: Bubble Tea v2, Bubbles v2 text input, Lip Gloss v2, Charmbracelet ANSI display-width utilities; existing custom form and viewport renderers

**Storage**: Existing SQLite catalog and native OS credential stores are unchanged; selector and hidden dependent values are transient in-memory form state

**Testing**: Go `testing`, table-driven package tests, model/view conformance tests, integration tests, fuzz tests, race tests, and build-tagged responsive acceptance tests

**Target Platform**: VT-capable terminals on Linux, macOS, and Windows; amd64 and arm64

**Project Type**: Local keyboard-first CLI/TUI application

**Performance Goals**: Preserve the existing target of at least 19/20 local input, focus, and resize updates completing within 100 ms; selector movement must update in the same rendered transition

**Constraints**: Keyboard-only; all three labels visible from the 40x12 minimum; no color-only selection; exactly one selected method; one focus stop; cyclic physical Left/Right navigation; all other selector text/edit/paste input inert; hidden dependent state retained but neither validated nor persisted; no secret projection; bounded output

**Scale/Scope**: One shared connection form used by create and edit, three fixed method options, nine method transitions, three required terminal sizes plus the existing twelve-size responsive matrix, color and no-color modes

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

### Pre-Research Gate

| Principle / Constraint | Evaluation | Result |
|------------------------|------------|--------|
| I. Secure SSH Operations | The selector displays only fixed non-secret labels and preserves existing SSH and host-trust behavior. A retained hidden remember choice is explicitly gated by the final `Password` selection so it cannot request or persist a secret for `Agent` or `Key`. | PASS |
| II. Terminal-First Usability | The control is keyboard-only, remains one focus stop, documents Left/Right navigation, preserves cancel/help/save/quit controls, fits at 40x12, and distinguishes focus from selection without color. | PASS |
| III. Predictable State and Failure Handling | Method, baseline, dependent values, focus fallback, and request projection have explicit transitions. Validation, persistence failure, resize, undersized recovery, and cancellation preserve or discard state according to the existing form owner. | PASS |
| IV. Behavior-Focused Testing | The plan covers create/edit initialization, cyclic movement, input isolation, all method transitions, dependent retention, request projection, credential gating, failures, responsive layout, no-color rendering, and principal regressions. | PASS |
| V. Simplicity and Maintainability | The existing form owns one typed selection and a fixed option inventory; no widget package, dependency, persistent state, background work, or parallel abstraction is introduced. | PASS |
| Security and operational constraints | No untrusted selector value reaches storage, no command or shell boundary changes, output remains bounded, and existing credential-store semantics remain authoritative. | PASS |
| Documentation and quality gates | README and contextual Help/Actions will describe the selector; formatting, tests, race detection, vet, static analysis, vulnerability checks, and Nix validation remain required. | PASS |

No constitutional violations or complexity exceptions are required.

### Post-Design Gate

Phase 1 preserves every pre-research result. The data model is transient and gives the selector exactly one valid typed value; the TUI contract fixes option order, focus and selection markers, cyclic keys, input isolation, dependent-state transitions, request projection, and responsive behavior. The quickstart validates main paths and principal failures without real hosts or secrets. No authentication protocol, storage schema, platform integration, dependency, or lifecycle owner changes. **Result: PASS.**

## Project Structure

### Documentation (this feature)

```text
specs/007-select-auth-method/
├── plan.md                         # This file (/speckit.plan output)
├── research.md                     # Phase 0 decisions
├── data-model.md                   # Phase 1 transient selector model
├── quickstart.md                   # Phase 1 validation guide
├── contracts/
│   └── tui-auth-method-selector.md # Phase 1 visual/input/state contract
└── tasks.md                        # Phase 2 output (/speckit.tasks; not created here)
```

### Source Code (repository root)

```text
internal/tui/
├── connection_form.go              # Typed method state, input, rendering, validation, dirty/request projection
├── connection_form_test.go         # Selector, dependencies, requests, failures, and input-isolation tests
├── model.go                        # Form paste routing, Help content, save and effective remember intent
├── actions.go                      # Contextual selector navigation descriptor
├── ui_conformance_test.go          # Repeated create/edit, focus, save, and responsive flows
├── resize_conformance_test.go      # Selector and hidden-state preservation through resize
├── embedded_form_security_test.go  # Printable/paste isolation and non-secret rendering
├── accessibility_test.go           # No-color focus/selection semantics
└── language_test.go                # Controlled English vocabulary

internal/app/
└── types.go                        # Existing AuthMethod and request types; unchanged contract

internal/domain/
└── connection.go                   # Existing authentication invariants; unchanged contract

tests/integration/
└── tui_connection_test.go          # Public keyboard create/edit persistence flows

README.md                           # User-visible selector, keys, paste boundary, and dependent controls
```

**Structure Decision**: Preserve the single Go module and existing `internal/tui` ownership. `connectionForm` remains the sole owner of form selection and dependent values; `model.go` retains operation and credential ownership; app/domain types and repository formats remain unchanged. Existing tests are adapted in place rather than creating a parallel component package.

## Complexity Tracking

> **Fill ONLY if Constitution Check has violations that must be justified**

No violations require justification.

## Implementation Sequence

1. Add the fixed ordered option inventory and typed selected/baseline method state while retaining `fieldAuth` as the stable form-row and focus identifier.
2. Replace method text rendering with a compact one-row selector whose bracket marker identifies selection independently from the existing row-focus marker.
3. Route only unmodified physical Left/Right to cyclic selection; consume printable, deletion, selection-modifier, and paste input while preserving global form controls.
4. Drive dependent visibility, validation, dirty state, create/update requests, and focus fallback from the typed method. Retain hidden dependent state, but project only applicable data.
5. Gate password credential intent on the final selected method and keep changed-away-from-key identity clearing behavior intact.
6. Add contextual selector keys to Actions/Help, update controlled-language fixtures, and revise README text/paste documentation.
7. Replace auth text-field tests with selector tests, extend create/edit request and credential regressions, and run focused, complete, race, responsive, and Nix quality gates.
