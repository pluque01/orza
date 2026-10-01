# Implementation Plan: Filter Tree Connections

**Branch**: `015-filter-tree-connections` | **Date**: 2026-10-01 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `/specs/015-filter-tree-connections/spec.md`

## Summary

Add a temporary, keyboard-driven regular-expression filter to the focused connection tree. The TUI will retain the catalog snapshot as the source of truth, derive a filtered tree projection containing only matching connections and their ancestor folders, and preserve the original selection, folder expansion, and viewport for cancellation. A shared text field will own query entry; filter-specific key handling will take priority while it is focused. Confirming a selected result restores the unfiltered tree, expands its folder path, and focuses the connection without dispatching a connection action.

## Technical Context

**Language/Version**: Go 1.26.0, toolchain go1.26.6

**Primary Dependencies**: Bubble Tea v2.0.8, Bubbles v2.1.1 text input, Lip Gloss v2.0.5, Charmbracelet ANSI utilities; Go standard-library regular expressions

**Storage**: Existing SQLite-backed catalog snapshot; no persistence or schema changes

**Testing**: Go standard testing, TUI unit/conformance tests, and integration tests in `tests/integration`

**Target Platform**: Interactive ANSI/VT terminal on the project-supported platforms

**Project Type**: Go CLI with an interactive terminal user interface

**Performance Goals**: Reproject a 100-connection nested tree within 1 second after a valid query change

**Constraints**: Keyboard-only operation; explicit Escape cancellation; preserve prior tree state; valid queries are case-sensitive regular expressions; invalid expressions cannot change selection or trigger operations; no color mode must retain a distinct active-result indicator

**Scale/Scope**: Existing in-memory catalog snapshot, at least 100 connections across nested folders; changes limited to the TUI tree and its tests

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Gate | Status | Evidence |
|------|--------|----------|
| Secure SSH operations remain unchanged | Pass | The filter reads only the loaded catalog snapshot and does not expose, persist, or act on credentials or sessions. |
| Terminal-first usability and cancellation | Pass | `/` opens a keyboard-focused field; Escape restores the captured tree context; arrow keys and Ctrl+P/Ctrl+N navigate results. |
| Predictable state and failure handling | Pass | Search is transient; invalid expressions retain the unfiltered tree and show an actionable error; Enter is selection-only. |
| Behavior-focused testing | Pass | Unit, conformance, and integration coverage will exercise input, valid/invalid expressions, hierarchy, navigation, cancellation, acceptance, color/no-color, and resize behavior. |
| Simplicity and maintainability | Pass | Reuse the existing text field, browser snapshot, expansion map, viewport, and style system; add no dependency, service, storage, or background work. |

**Post-design review**: Pass. The data model and UI contract keep filtering within the root TUI model and browser projection. No constitution exception or complexity justification is required.

## Project Structure

### Documentation (this feature)

```text
specs/015-filter-tree-connections/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
```text
cmd/orza/                         # CLI entry point
internal/tui/
├── model.go                      # root state and key-routing changes
├── browser.go                    # filtered tree projection and match rendering
├── keys.go                       # search and filtered-navigation bindings
├── text_field.go                 # reused query editor
├── styles.go                     # query-match presentation style
└── *_test.go                     # unit and conformance coverage

tests/integration/
└── tui_tree_test.go              # end-to-end catalog/tree filtering coverage
```

**Structure Decision**: Keep this feature in the existing `internal/tui` package. The root model owns transient search lifecycle and input priority; `browserModel` derives display rows from its existing catalog snapshot. No new package is warranted.
