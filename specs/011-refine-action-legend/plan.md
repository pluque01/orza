# Implementation Plan: Refine Action Legend

**Branch**: `011-refine-action-legend` | **Date**: 2026-09-24 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/011-refine-action-legend/spec.md`

## Summary

Replace the non-interactive Actions panel in the connection browser with a compact, borderless legend that only shows navigation, optional Connect, creation, and exit actions. Keep the complete contextual action inventory in Help, rendered as ordered groups of aligned key/action pairs. Reuse the existing action descriptors for availability and dispatch, add a presentation-only legend selector, and reuse the Details field presentation for Help's aligned rows.

## Technical Context

<!--
  ACTION REQUIRED: Replace the content in this section with the technical details
  for the project. The structure here is presented in advisory capacity to guide
  the iteration process.
-->

**Language/Version**: Go 1.26.0 (toolchain go1.26.6)

**Primary Dependencies**: Bubble Tea v2, Bubbles v2, Lip Gloss v2, Charmbracelet ANSI utilities

**Storage**: N/A; this feature does not change persisted connection metadata

**Testing**: Go standard testing package; package-local, table-driven TUI and layout tests

**Target Platform**: Supported terminal environments on the project's supported desktop platforms; normal browser layout supports terminals at least 40x12

**Project Type**: Terminal user interface within a CLI SSH connection manager

**Performance Goals**: Recompute and render the legend and Help synchronously within the next visible TUI state after a selection or focus change

**Constraints**: Preserve all existing key dispatch and contextual availability; color is supplemental; Help controls remain visible while its content scrolls; use display-cell widths for alignment and narrow layouts

**Scale/Scope**: One browser legend and its Help presentation; no new actions, key bindings, persistence, external interfaces, or dependencies

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principle | Gate | Status |
|-----------|------|--------|
| Terminal-First Usability | Keyboard operations stay intact, Help and Quit remain discoverable, and the interface is understandable without color at 40x12 or above. | Pass |
| Predictable State and Failure Handling | This is presentation-only; action applicability and dispatch remain derived from the existing contextual inventory. | Pass |
| Behavior-Focused Testing | Add automated coverage for legend contents/order, Help coverage/order/alignment, color stripping, narrow layouts, and hidden-action dispatch. | Pass |
| Simplicity and Maintainability | Reuse descriptors, style primitives, and structured field rendering; do not add dependencies or persistent state. | Pass |
| Secure SSH Operations | No credential, connection protocol, host-verification, or persistence behavior changes. | Pass |

**Post-design re-check**: Pass. The design retains the existing action dispatch boundary, adds no external calls or state, and covers the keyboard and no-color requirements with automated tests.

## Project Structure

### Documentation (this feature)

```text
specs/011-refine-action-legend/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/           # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
<!--
  ACTION REQUIRED: Replace the placeholder tree below with the concrete layout
  for this feature. Delete unused options and expand the chosen structure with
  real paths (e.g., apps/admin, packages/something). The delivered plan must
  not include Option labels.
-->

```text
cmd/orza/                     # CLI entry point
internal/tui/
├── actions.go                # Action descriptors, legend selection, Help rows
├── actions_test.go            # Action and Help presentation tests
├── action_dispatch_test.go    # Dispatch behavior tests
├── detail.go                  # Existing structured Details field usage
├── layout.go                  # Browser geometry and minimum-size layout
├── layout_test.go             # Geometry matrix tests
├── responsive_layout_test.go  # Narrow and wide layout tests
├── modal.go                   # Help modal rendering and scrolling
├── modal_layout_test.go       # Help modal geometry tests
├── model.go                   # Browser shell and Help opening
├── styles.go                  # Semantic text styles and structured fields
├── styles_test.go             # Color and display-width tests
└── ui_conformance_test.go     # Visible UI and dispatch conformance
specs/011-refine-action-legend/
├── contracts/tui-actions.md
├── data-model.md
├── plan.md
├── quickstart.md
├── research.md
└── spec.md
```

**Structure Decision**: Keep the existing single Go CLI project. Limit production changes to `internal/tui`, where action applicability, rendering, layout, and Help modal behavior already reside; retain package-local tests alongside those files.
