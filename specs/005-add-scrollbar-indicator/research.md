# Phase 0 Research: Indicador de barra de desplazamiento

## Shared Viewport Strategy

**Decision**: Extend the existing custom viewport projection in `internal/tui/viewport.go` rather than adopting another component or maintaining surface-specific scrollbar calculations. The projection will expose effective visible-range and scrollbar metadata alongside projected lines.

**Rationale**: Tree, Details, forms, Help, pickers, confirmations, and recoverable errors already converge on the same integer-offset projection rules. A shared pure calculation preserves consistent active-line visibility, logical-offset restoration, Unicode-safe truncation, and deterministic tests without adding a dependency or parallel state owner.

**Alternatives considered**:

- Replace the custom viewport with Bubbles viewport: rejected because forms, active tree rows, modal priority controls, and logical-offset restoration use project-specific projection behavior that would still require adapters.
- Calculate bars independently in each renderer: rejected because range and rounding behavior would diverge and duplicate edge-case handling.
- Add persistent scrollbar state: rejected because geometry is derived completely from current viewport state and dimensions; persisted derived state could become stale.

## Effective Offset and Visible Range

**Decision**: Derive scrollbar position from the projection's effective `renderOffset`, not from retained `logicalOffset`. Preserve `logicalOffset` unchanged when resize or active-line visibility temporarily clamps the rendered range.

**Rationale**: Tree selection, form focus, and move-picker selection can force an effective range without changing the retained offset. The indicator must describe what is actually visible while preserving the existing restoration behavior when the viewport grows again.

**Alternatives considered**:

- Use `logicalOffset`: rejected because the thumb can disagree with visible tree/form content.
- Mutate the logical offset whenever clamped: rejected because it breaks exact state restoration after resize.

## Proportional Geometry and Rounding

**Decision**: For track height `T`, total final content rows `N`, visible capacity `V`, effective offset `O`, and maximum offset `M = N - V`, calculate:

- Thumb length `L = clamp(1, T, roundHalfUp(T × V ÷ N))`.
- Thumb top `P = roundHalfUp((T - L) × O ÷ M)`.
- Hide the bar when `N <= V`; clamp inputs before calculation.

Use one shared non-negative multiply-divide helper that cannot overflow before division. The helper may use wide integer multiplication/division from the standard library or an equivalent quotient/remainder decomposition proven for all accepted `int` inputs.

**Rationale**: The formulas directly implement FR-005 and FR-006, guarantee exact top and bottom endpoints, and keep thumb length stable while scrolling. Explicit half-up rounding removes tie ambiguity. Safe arithmetic preserves deterministic behavior under fuzzed or extreme dimensions.

**Alternatives considered**:

- Floating-point percentages: rejected because they introduce avoidable rounding differences and are unnecessary for integer terminal cells.
- Truncating division: rejected because it systematically biases thumb size and position toward the top.
- Direct `int` multiplication: rejected because overflow can occur before division even when the final result fits.
- Cumulative start/end proportions: rejected because quantization can make thumb length vary by offset.

## One-Row and Invalid Geometry

**Decision**: A one-row overflowing track shows one thumb cell at its only position. A zero-height track, zero content, or zero visible capacity produces no bar. Stale offsets and active lines are clamped for rendering without mutating retained logical state.

**Rationale**: One terminal row cannot encode position and proportion simultaneously, but a visible thumb still communicates overflow. Invalid or stale geometry must remain bounded and must never panic or draw outside a panel.

**Alternatives considered**:

- Hide a one-row scrollbar: rejected because it loses the only overflow cue.
- Reintroduce directional text in short tracks: rejected because it violates the replacement requirement and consumes content rows.

## Conditional Padding Cell

**Decision**: Keep the established content width unchanged and replace the existing right-padding cell with the track/thumb only while overflow exists. When content fits, the same cell remains ordinary padding.

**Rationale**: `layoutRect.contentWidth()` already excludes one padding cell on each side. Reusing the right cell places the bar directly beside the border without changing wrapping, form widths, outer rectangles, or neighboring panels.

**Alternatives considered**:

- Always reserve a track column: rejected because fitting content must not show an empty track.
- Reduce content width by one: rejected because it would reserve two cells between content and the border and create unnecessary reflow.
- Change outer panel dimensions: rejected because it would alter the responsive layout contract.

## Panel Composition and Track Extent

**Decision**: Carry scrollbar metadata with each projected content section. The region compositor will render the bar in the right-padding cell immediately adjacent to the border, while projected text is limited to the remaining width. Track rows cover only the section's scrollable rows.

**Rationale**: The current shell always inserts right padding before the border. Reusing that cell preserves outer geometry while meeting the adjacency requirement. Details can have a fixed notice before content, forms can have fixed conflict text, and modals have fixed recovery/control rows; decorating the entire panel would incorrectly imply those rows scroll.

**Alternatives considered**:

- Change outer panel dimensions: rejected because it risks responsive-layout regressions and is not required.
- Infer track rows from final rendered strings: rejected because fixed and scrollable sections are indistinguishable after composition.
- Span modal controls with the track: rejected because controls are intentionally fixed and must remain visible.

## Track and Thumb Presentation

**Decision**: Use distinct one-cell textual glyphs for track and thumb, with semantic styles added by the existing style set. The planned contract uses `│` for track and `█` for thumb, with `|` and `#` as safe fallbacks if a configured glyph is not one display cell. Width validation uses the project's ANSI/display-width utilities.

**Rationale**: Shape and density remain distinguishable when color is disabled. Both pairs are conventional terminal primitives, require no mouse semantics, and fit the one-column contract.

**Alternatives considered**:

- Different colors on the same glyph: rejected because color cannot be the only cue.
- Background-colored spaces: rejected because no-color terminals make the track disappear.
- Thin/heavy line pair: rejected because the visual distinction is weaker across fonts than line/block.

## Non-Scrollable Overflow Text

**Decision**: Do not add scrollbars to Actions or to final-frame clipping because neither exposes a scroll offset. Rename the Actions omission cue from `↓ more — ? Help` to `Hidden actions — ? Help`, and remove the final `fitContent` marker fallback in favor of bounded clipping. Help remains the keyboard-accessible complete action inventory.

**Rationale**: A proportional bar without a movable range would be misleading and would expand feature scope by requiring new focus or controls. The specification requires zero visible `↑ more`/`↓ more`, while preserving existing keyboard behavior and action priorities.

**Alternatives considered**:

- Make Actions scrollable: rejected because it changes focus and keyboard contracts and is explicitly outside the feature scope.
- Display an Actions scrollbar without controls: rejected because it implies inaccessible content.
- Keep the old text because it is not a viewport marker: rejected because SC-001 requires zero appearances.

## Test Strategy

**Decision**: Replace marker-oriented assertions with pure geometry tests and cross-surface right-edge contract tests, then retain and adapt existing navigation, resize, no-color, Unicode, fuzz, performance, and acceptance suites. No external terminal or SSH service is required for scrollbar tests.

**Rationale**: The changed behavior is deterministic rendering and can be tested through existing model/view boundaries. Pure calculations cover arithmetic boundaries; surface tests prove composition and fixed-row behavior; existing suites protect focus, state restoration, and bounded output.

**Alternatives considered**:

- Snapshot-only tests: rejected because ANSI and responsive output make failures difficult to localize and do not prove formula invariants.
- Manual terminal validation only: rejected by the constitution's behavior-focused testing requirement.

## Dependency and Platform Decision

**Decision**: Keep Go 1.26 with the pinned Bubble Tea v2, Lip Gloss v2, and ANSI width dependencies. Add no production dependency. Target the existing VT-capable Linux, macOS, and Windows terminal matrix and preserve keyboard-only operation.

**Rationale**: Existing packages already provide rendering, styling, and display-cell measurement. The feature does not touch storage, SSH lifecycle, credentials, or platform integration.

**Alternatives considered**:

- Add a scrollbar package: rejected because the required calculation and one-column composition are small, project-specific, and dependency-free.
