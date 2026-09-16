# Data Model: Estilo visual unificado de paneles

This feature changes no persistent schema, domain entity, external data format, or operation state. It introduces transient presentation descriptors derived from existing TUI state during deterministic rendering.

## Type Badge

Identifies a panel or a distinct content type within a panel.

| Field | Type | Rules |
|-------|------|-------|
| `label` | controlled English text | Non-empty, application-owned, contains no secret or catalog-derived value. Examples: `Tree`, `Details`, `Connection`, `Help`, `Operation Error`. |
| `placement` | title or content | Title when the current panel title already identifies the type; content only when the container and content types differ. |
| `active` | derived boolean | For focus-owning region titles only; preserves the existing active/inactive textual marker. |
| `accented` | derived boolean | True in color mode; false under `--no-color` or `NO_COLOR`. Does not change the plain badge text. |

### Invariants

- The plain representation is bracket-delimited, for example `[Connection]`.
- Color mode styles the same delimited text; stripping ANSI yields the no-color representation exactly.
- One semantic type appears once at a given panel level.
- Details may contain both `[Details]` and `[Connection]` because they identify the region and selected content respectively.
- A badge never contains user-controlled data, target identity, a credential reference, or secret bytes.

## Display Field

Represents one ordered label/value association on a structured TUI surface.

| Field | Type | Rules |
|-------|------|-------|
| `label` | controlled English text | Non-empty; rendered without a trailing colon; styled as secondary text in color mode. |
| `value` | safe visible text or component view | Sanitized before styling when dynamic; may contain value punctuation such as `host:port`. |
| `disclosure` | wrap or truncate | Inherited from the existing surface contract; confirmation targets wrap fully, ordinary bounded rows may truncate safely. |
| `semantics` | optional existing row semantics | Focused, invalid, primary, selected, or read-only; existing text markers remain authoritative. |

### Invariants

- Display order follows the current surface order.
- Label punctuation is not part of the stored descriptor.
- A value does not begin before the shared value column in aligned mode.
- Removing colons does not remove status words, focus markers, validation markers, or action controls.
- Dynamic values pass through the existing control, bidi, UTF-8, and display-width safety boundary before style is applied.

## Structured Field Group

Provides one layout decision for an ordered collection of Display Fields within a local content width.

| Field | Type | Rules |
|-------|------|-------|
| `fields` | ordered Display Fields | Zero or more fields belonging to one logical surface block. |
| `labelWidth` | display cells | Maximum visible width of field labels in the group. |
| `prefixWidth` | display cells | Existing marker space required by the surface, such as form focus/validity/primary slots. |
| `contentWidth` | display cells | Width available inside the current region or modal after borders, padding, and scrollbar reservation. |
| `mode` | aligned, stacked, or compact-interactive | Derived from content role and available value width; never persisted. |
| `minimumValueWidth` | display cells | Eight cells for a non-interactive identity group's aligned-mode decision; interactive components retain their existing bounded minimum. |

### Derived Layout

- `availableValueWidth = contentWidth - prefixWidth - labelWidth - 1`.
- A non-interactive identity group uses `aligned` when `availableValueWidth >= 8` and `stacked` otherwise.
- An interactive form uses `compact-interactive` at supported terminal sizes so one field row plus its validation error can remain visible at 40x12.
- Aligned mode emits one visible row per field: padded label, one separating space, then value.
- Stacked mode emits a label line followed by its value line and does not omit a field.
- Compact-interactive mode keeps label and bounded component value on one row while retaining the shared label styling and best available value column.
- Long values follow their field's disclosure policy after the mode is selected.

## Panel Presentation

Describes the transient visual composition of one existing surface.

| Field | Type | Rules |
|-------|------|-------|
| `containerBadge` | Type Badge | Existing region or modal title rendered as a badge. |
| `contentBadge` | optional Type Badge | Present only when the current content type differs from the container type. |
| `fieldGroups` | zero or more Structured Field Groups | Structured identity, form, confirmation, warning, or diagnostic content. |
| `body` | existing visible rows | Help text, action descriptors, picker choices, questions, explanations, and empty states. |
| `controls` | existing priority rows | Current keyboard controls; inventory and order are unchanged. |
| `viewport` | existing viewport state | Existing logical offset, active row, and scrollbar geometry. |

### Relationships

- One visible region or modal has one container badge.
- One Panel Presentation may have one distinct content badge.
- One Panel Presentation owns zero or more field groups but does not own their source domain data.
- Existing model, form, modal, prompt, and viewport owners remain responsible for state and interaction.

## State Transitions

### Render or Content Update

1. Read existing immutable view state for the current frame.
2. Select controlled badge labels for the container and distinct content.
3. Project dynamic values through existing safety rules.
4. Compute field-group layout from local content width.
5. Apply semantic styles without changing plain text cues.
6. Project through the existing active-row and viewport rules.

### Resize Within Supported Dimensions

1. Preserve selection, focus, errors, modal payload, form drafts, and logical viewport offset.
2. Recompute local content widths.
3. Recompute non-interactive groups between aligned and stacked mode while retaining compact interactive rows.
4. Reproject active blocks and fixed controls in the same visible update.

### Enter Undersized Mode

1. Preserve all existing interaction state.
2. Render only the existing terminal-too-small surface below 40x12.
3. Do not render partial badges or structured panels.

### Restore Supported Dimensions

1. Restore the same state and owners.
2. Recompute badges and field modes for the restored dimensions.
3. Show the active field, target identity, recovery controls, and primary action according to existing priority rules.

### Toggle Color Mode at Startup

1. Use the same controlled badge, label, value, and marker text.
2. Apply accent and muted styles only when color is enabled.
3. Emit no ANSI styling when color is disabled.
