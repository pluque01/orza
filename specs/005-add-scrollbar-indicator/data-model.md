# Data Model: Indicador de barra de desplazamiento

This feature adds no persistent business data. All entities below are transient values derived during one deterministic TUI render.

## Viewport State

Represents the user-controlled logical position already retained by each scrollable surface.

| Field | Type | Rules |
|-------|------|-------|
| `logicalOffset` | non-negative integer | Preserved across resize; reset only by existing target/lifecycle rules; never mutated solely to calculate a scrollbar. |

### Validation

- Negative values normalize to zero for projection.
- A value beyond the current maximum is clamped only for the effective render range.
- An active row or active block may override the effective range without overwriting this state.

## Viewport Input

Describes one projection request after content generation.

| Field | Type | Rules |
|-------|------|-------|
| `lines` | ordered text rows | Terminal-safe source rows generated for the selected width. |
| `availableRows` | integer | Number of rows available to scrollable content; must be non-negative. |
| `availableWidth` | integer | Width before conditional scrollbar reservation; must be non-negative. |
| `activeRange` | optional inclusive row range | Selected tree row, focused field/error block, or active picker row that must remain visible. |
| `state` | Viewport State | Source of retained logical offset. |

### Validation

- Active bounds are clamped to existing content.
- Empty content has no active range and no scrollbar.
- Fixed panel notices and modal controls are excluded from `lines` and composed separately.
- Zero available rows or an interior width that cannot preserve one content cell produces no scrollbar geometry and delegates presentation to existing reduced-layout priorities.

## Viewport Projection

Immutable result used both for rendering content and describing the visible range.

| Field | Type | Rules |
|-------|------|-------|
| `lines` | ordered visible text rows | Contains content only; directional marker rows no longer exist. |
| `logicalOffset` | non-negative integer | Echoes the retained state for future restoration. |
| `renderOffset` | non-negative integer | Effective first visible source row after clamping and active visibility. |
| `contentLength` | non-negative integer | Number of source rows after final-width reflow. |
| `visibleRows` | non-negative integer | Number of content rows represented by the final range, bounded by capacity and content. |
| `availableRows` | non-negative integer | Final track/content capacity. |
| `activeRange` | optional range | Effective active rows used to derive visibility. |
| `scrollbar` | optional Scrollbar Geometry | Present exactly when `contentLength > availableRows` and capacity is positive. |

### Invariants

- `0 <= renderOffset <= max(0, contentLength - availableRows)`.
- Every returned line fits the final content width by display cells.
- An existing active range intersects the visible range whenever it can fit; oversized active blocks prioritize their existing terminal rows and report the actual first rendered row.
- Projection does not mutate state, perform I/O, or retain prior frames.

## Scrollbar Geometry

Pure geometry associated with one final viewport projection.

| Field | Type | Rules |
|-------|------|-------|
| `trackHeight` | positive integer | Equals the scrollable section's available rows. |
| `thumbTop` | non-negative integer | Zero-based first thumb row within the track. |
| `thumbLength` | positive integer | At least one and at most `trackHeight`. |
| `trackGlyph` | one-display-cell text | `│` by default; falls back to `|` if width validation fails. |
| `thumbGlyph` | one-display-cell text | `█` by default; falls back to `#` if width validation fails. |

### Derived Rules

For `T = trackHeight`, `N = contentLength`, `V = availableRows`, `O = renderOffset`, and `M = N - V`:

- `thumbLength = clamp(1, T, roundHalfUp(T × V ÷ N))`.
- `thumbTop = roundHalfUp((T - thumbLength) × O ÷ M)`.
- All multiplication/division is overflow-safe for accepted integers.
- At offset zero, `thumbTop = 0`.
- At maximum offset, `thumbTop + thumbLength = trackHeight`.
- For a one-row track, `thumbTop = 0` and `thumbLength = 1`.
- Quantized intermediate offsets may share an endpoint cell when `trackHeight - thumbLength` is too small to represent every logical position.

## Projected Section

Carries a viewport projection through panel composition without confusing fixed and scrollable rows.

| Field | Type | Rules |
|-------|------|-------|
| `leadingFixedLines` | ordered rows | Optional notice/conflict/header rows before the viewport; never receive track cells. |
| `viewport` | Viewport Projection | Scrollable content and optional bar. |
| `trailingFixedLines` | ordered rows | Optional modal recovery/control rows; never receive track cells. |

### Relationships

- Tree and Details each create one projected section.
- A connection form creates one section whose active range can span a field and its validation lines.
- A modal creates one section with a scrollable body and fixed trailing priority controls.
- Actions does not create a projected section because it is packed, non-focusable, and non-scrollable.

## Render Transitions

### Content fits to overflow

1. Full-width first pass reports `contentLength > availableRows`.
2. Projection clamps the effective range and creates geometry from the same content width.
3. Composition replaces the existing right-padding cell with track/thumb.

### Overflow to content fits

1. Full-width first pass reports `contentLength <= availableRows`.
2. No scrollbar geometry is produced.
3. The rightmost interior cell returns to normal padding.
4. Logical state remains retained even if the effective range is currently clamped.

### Scroll

1. Existing keyboard handling changes logical offset or active selection/focus.
2. Projection derives a new effective range.
3. Content and indicator update in the same frame.

### Content shrink or resize

1. Retained logical state remains unchanged.
2. Effective offset clamps to the nearest valid range while keeping active content visible.
3. Geometry uses the clamped effective offset.
4. A later compatible size may restore the retained logical position under existing rules.

### Undersized view

1. Below 40x12, normal regions are not projected or composed.
2. No scrollbar geometry exists in the rendered frame.
3. Viewport states remain intact.
4. Returning to a supported size derives applicable bars from restored state.
