# Implementation Plan: Hide Connection Actions

**Branch**: `013-hide-connection-actions` | **Date**: 2026-09-25 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/013-hide-connection-actions/spec.md`

## Summary

Remove the titled Actions panel from every TUI state while retaining the fixed lower control area and the applicable borderless action legend. Use the existing action descriptors and keyboard dispatch unchanged; render their status and controls without a panel title or border. Preserve the fixed control-area geometry at normal and minimum supported terminal sizes rather than reallocating it to Tree or Details.

## Technical Context

**Language/Version**: Go 1.26.0 (toolchain Go 1.26.6)

**Primary Dependencies**: Bubble Tea v2, Bubbles v2, Lip Gloss v2, Charmbracelet ANSI utilities

**Storage**: N/A; this feature changes only transient TUI presentation

**Testing**: Go unit, conformance, resize, accessibility, and integration tests via `go test ./...`

**Target Platform**: Cross-platform terminal application on supported Linux, macOS, and Windows terminals

**Project Type**: Terminal user interface within a CLI application

**Performance Goals**: Preserve the existing single-render interaction responsiveness and bounded rendering at the 40x12 minimum terminal size

**Constraints**: No Actions panel title or border in any user-visible workflow; preserve a fixed lower control area without redistributing it; retain applicable action legends and all current keyboard behavior

**Scale/Scope**: One local interactive session; catalog, form, modal, connection-startup, conflict, recovery, and error views

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Pre-design gate: PASS**

- Secure SSH Operations: PASS. Host identity prompts, trust decisions, and secret handling are presentation-adjacent but unchanged.
- Terminal-First Usability: PASS. The action legend and keyboard cancellation, back, and quit controls remain visible and operable at 40x12.
- Predictable State and Failure Handling: PASS. No connection lifecycle or ownership transition changes; startup and recovery controls retain their current dispatch.
- Behavior-Focused Testing: PASS. Add regression coverage for panel absence, legend presence, and startup/recovery behavior.
- Simplicity and Maintainability: PASS. Reuse the existing fixed control layout, descriptor inventory, and rendering utilities; add no dependency, persistence, or background work.

## Project Structure

### Documentation (this feature)

```text
specs/013-hide-connection-actions/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
```text
internal/tui/
├── actions.go                       # action descriptors and borderless legends
├── layout.go                        # fixed lower control-region geometry
├── model.go                         # browser-shell presentation selection
├── operation_integration_test.go    # operation/startup presentation tests
├── layout_test.go                   # geometry tests
├── responsive_layout_test.go         # minimum-size and resize coverage
├── accessibility_test.go             # keyboard and readable-control coverage
└── *_conformance_test.go             # state, recovery, and viewport matrices

tests/integration/
└── tui_actions_test.go               # end-to-end TUI action visibility coverage
```

**Structure Decision**: Modify the existing `internal/tui` presentation and layout paths. No new module, data store, or external interface is needed.

## Complexity Tracking

No constitution violations or complexity exceptions.

## Constitution Check (Post-Design)

**Post-design gate: PASS**

- The design retains host-trust, authentication, cancellation, and quit controls without changing their state transitions.
- The fixed lower region avoids layout reflow and remains readable at the 40x12 minimum size.
- The design is limited to presentation and regression tests, with no new dependencies, persistence, or concurrency.
