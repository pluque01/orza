# TUI Contract: Proportional Scrollbar

This contract supersedes the vertical `↑ more`/`↓ more` marker rules in feature 004. Horizontal `…` truncation, keyboard bindings, focus ownership, region geometry, and content priority remain unchanged unless explicitly stated here.

## Visual Vocabulary

| Element | Color-capable rendering | No-color rendering | Display width |
|---------|-------------------------|--------------------|---------------|
| Track | Muted `│` | `│` | Exactly 1 cell |
| Thumb | Focus/status-styled `█` | `█` | Exactly 1 cell |
| Safe track fallback | Muted `|` | `|` | Exactly 1 cell |
| Safe thumb fallback | Emphasized `#` | `#` | Exactly 1 cell |

- Track and thumb MUST remain distinguishable by shape/density after ANSI styling is removed.
- Glyphs MUST be validated with the same display-width semantics used for content truncation.
- The bar MUST NOT be identified by color alone.

## Placement

For a normal bordered panel with vertical overflow:

```text
┌─ Details ──────────┐
│ first visible row █│
│ next visible row  █│
│ next visible row  ││
│ last visible row  ││
└────────────────────┘
```

The penultimate cell on every content row is the scrollbar track or thumb; the final `│` is the unchanged outer right border immediately after it.

- The bar consumes exactly one interior column only while overflow exists.
- The bar occupies the cell immediately left of the right border, replacing the normal right-padding cell.
- Text keeps the established content width and never crosses the existing right-padding column used by the scrollbar.
- Outer rectangles, borders, titles, gutters, and neighboring regions do not move when the bar appears.
- When content fits, the bar is absent and content reclaims the column.

## Track Extent

- Tree: every Tree content row belongs to the track.
- Details: only the projected detail rows belong to the track; any fixed navigation notice before them does not.
- Connection form: only projected form rows belong to the track; fixed conflict/recovery context outside the form projection does not.
- Modal: only scrollable body rows belong to the track; fixed recovery, cancel, back, quit, and primary-control rows do not.
- Move picker: destination rows use the modal-body track and the active destination stays visible.
- Actions: no track; Actions remains non-focusable and non-scrollable.
- Undersized view below 40x12: no track because normal containers are replaced.

Example modal:

```text
┌─ Help ─────────────┐
│ body row 1        █│
│ body row 2        ││
│ body row 3        ││
│ Esc Close          │
└────────────────────┘
```

The fixed `Esc Close` row has no track cell and remains visible at every body offset.

## Visibility and Width Reflow

Each width-sensitive surface follows one deterministic policy:

1. Generate content using the established content width, which excludes left and right padding.
2. If total rows do not exceed available rows, keep the right-padding cell and render no bar.
3. If total rows exceed available rows, replace the right-padding cell with the track/thumb.
4. Calculate range and geometry from the same content rows; scrollbar visibility never changes wrapping width.

## Geometry

Let:

- `T` be track rows.
- `N` be final source rows after reduced-width regeneration.
- `V` be visible row capacity.
- `O` be the clamped effective first visible row.
- `M = N - V` be maximum effective offset.

When `N > V > 0` and `T > 0`:

```text
thumbLength = clamp(1, T, roundHalfUp(T * V / N))
thumbTop    = roundHalfUp((T - thumbLength) * O / M)
```

- Exact halves round toward the greater non-negative integer.
- Calculations MUST avoid intermediate integer overflow.
- `O` uses the effective rendered range after active-row and bounds clamping.
- At the first range, the thumb touches the top.
- At the last range, the thumb touches the bottom.
- At intermediate ranges, the thumb remains inside the track and moves in the same direction as content.
- Quantized intermediate ranges may share a top or bottom cell with an endpoint when the available thumb travel cannot represent every effective offset.
- A one-row overflowing track contains one thumb cell and communicates only that overflow exists, not relative position or proportion.
- A zero-row body or an interior width that cannot preserve at least one content cell beside the bar has no scrollbar and follows the existing reduced/undersized content priorities.

## Input Contract

- The scrollbar receives no focus, hover, selection, click, drag, wheel, or other input.
- Existing `Up`/`k`, `Down`/`j`, `Home`/`g`, and `End`/`G` behavior remains unchanged for the current focus owner.
- Existing `Tab`, `Shift+Tab`, and `F2` focus behavior remains unchanged.
- Help describes only existing keyboard controls and does not advertise mouse behavior.
- No new Bubble Tea mouse mode or terminal mouse capability is enabled.

## State Contract

- Each scrollable surface derives its own geometry from its own final content, dimensions, and effective offset.
- Moving Tree MUST NOT change the Details or background modal offset except through existing target-change rules.
- Opening or scrolling a modal MUST NOT alter background Tree/Details offsets.
- Selection, focus, form values, errors, active target, and operation ownership are unchanged by rendering a bar.
- Resize can clamp the effective range but MUST preserve the retained logical offset under existing restoration rules.
- Content shrink clamps the effective range to the nearest valid position and keeps active content visible.

## Overflow Text Removal

- Scrollable containers MUST render zero `↑ more` and zero `↓ more` rows.
- Actions MUST use `Hidden actions — ? Help` when lower-priority descriptors are omitted; it MUST NOT gain scrolling or a scrollbar.
- Final-frame bounds enforcement MUST clip safely without inserting either directional `more` label.
- Horizontal line truncation continues to use `…` inside the assigned text width.

## Accessibility and Safety

- The track/thumb distinction survives `--no-color` and `NO_COLOR`.
- A bar never replaces selected rows, focused fields, validation errors, active error/recovery text, primary controls, cancel/back/quit controls, or action targets.
- Untrusted text remains sanitized and bounded before composition; scrollbar glyphs are application-controlled.
- The final rendered frame remains within the reported terminal viewport.

## Contract Test Matrix

| Surface | Fit | Start | Middle | End | Resize fit↔overflow | No color | Active visibility |
|---------|-----|-------|--------|-----|---------------------|----------|-------------------|
| Tree | Required | Required | Required | Required | Required | Required | Selected row |
| Details | Required | Required | Required | Required | Required | Required | Current range |
| Connection form | Required | Required | Required | Required | Required | Required | Focus/error/Save block |
| Help | Required | Required | Required | Required | Required | Required | Fixed Close control |
| Move picker | Required | Required | Required | Required | Required | Required | Active destination |
| Confirmations | Required | Required | Required | Required | Required | Required | Full identity and Cancel |
| Recoverable errors | Required | Required | Required | Required | Required | Required | Error and recovery controls |

Every overflowing matrix cell verifies right-border adjacency, full body-track height, exact proportional geometry after half-up rounding, bounded output, and absence of directional `more` text.
