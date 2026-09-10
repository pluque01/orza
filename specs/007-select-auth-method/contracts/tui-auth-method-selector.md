# TUI Contract: Authentication Method Selector

This contract replaces free-text method entry in create and edit connection forms. Existing connection-form ownership, focus traversal, viewport/scrollbar behavior, error priority, operation lifecycle, and secret handling remain unchanged unless stated here.

## Closed Inventory

| Position | Visible label | Connection value | Dependent control |
|----------|---------------|------------------|-------------------|
| 1 | `Agent` | `agent` | None |
| 2 | `Key` | `key` | `Identity file` |
| 3 | `Password` | `password` | `Remember password` |

- The selector MUST contain exactly these options in this order.
- Labels are controlled English text and MUST NOT be edited, translated, pasted over, or derived from user/catalog input.
- A form MUST have exactly one selected method at all times.
- A new form selects Agent; an edit form selects the captured connection's method.

## Visual Contract

The Method row displays all options simultaneously. Brackets mark the selected option; the existing leading row marker identifies focus independently.

```text
  Method: [Agent] Key Password
> Method: Agent [Key] Password
  Method: Agent Key [Password]
```

- The selected label MUST be bracketed in color and no-color modes.
- Exactly one label MUST be bracketed.
- The existing `>` form-row marker MUST appear only when Method owns focus.
- Color MAY emphasize focus or selection but MUST NOT replace either textual marker.
- All three complete labels and the selected marker MUST fit within the Method row at every supported terminal size from 40x12 upward.
- The selector is one projected form row and one form focus position.

## Input Contract

When Method owns focus:

| Input | Required result |
|-------|-----------------|
| Unmodified Left | Select the previous option; Agent wraps to Password. |
| Unmodified Right | Select the next option; Password wraps to Agent. |
| Tab | Move to the next applicable form control. |
| Shift+Tab or unmodified F2 | Move to User. |
| Ctrl+S | Apply existing form save behavior. |
| Esc | Apply existing form cancel/unsaved behavior. |
| F1 | Open contextual form Help. |
| Ctrl+C | Apply existing safe-exit behavior. |
| Printable text, including `h`, `l`, `q`, `?`, and method names | Consume with no mutation and no command dispatch. |
| Backspace, Delete, Home, End, modified arrows | Consume with no mutation. |
| Paste, including newlines, controls, Unicode, or method names | Consume atomically with no mutation. |

- Selection changes in the same update as the accepted Left/Right input; Enter or Space confirmation is not required.
- Ignored selector input MUST NOT alter another field, cursor, selection, dependent draft, error, or outer browser state.
- No mouse, click, wheel, hover, or drag behavior is introduced.

## Focus Order

Forward order:

```text
Name → Host → Port → User → Method → applicable dependent control → Save
```

- Agent skips directly from Method to Save.
- Key includes Identity file immediately after Method.
- Password includes Remember password immediately after Method.
- Reverse traversal uses the exact opposite order.
- Existing full-form traversal wraps after Save/Name.
- Changing Method does not move focus away from Method.
- If a method change or restored state makes the currently focused dependent control inapplicable, Method receives focus immediately.

## Dependent State Contract

| Selected method | Identity file | Remember password |
|-----------------|---------------|-------------------|
| Agent | Hidden; not validated or saved | Hidden; no credential intent |
| Key | Visible, focusable, required, saved | Hidden; no credential intent |
| Password | Hidden; not validated or saved | Visible, focusable, unchecked initially |

- Identity and Remember drafts remain in memory when hidden and reappear unchanged if their method is selected again during the same form.
- Hidden drafts remain unsaved form data for cancel/quit protection.
- Only final selected-method data may be validated or projected into a request.
- Remember may request password input or persistence only when Password is the final method and the visible control is checked.
- Changing away from a persisted Key continues to clear stale identity metadata through the existing update contract.
- Changing away from a remembered Password continues to use the existing credential-removal lifecycle; the selector does not display or own secret bytes.

## Validation and Failure Contract

- Arbitrary invalid method text is unreachable through selector input.
- Existing domain validation remains authoritative at request/service boundaries.
- Key without a valid non-empty Identity produces the existing field-local actionable error.
- A validation, persistence, or conflict failure MUST preserve selected method, dependent drafts, focus, target, and captured revision.
- Failed save MUST NOT substitute Agent or any other method.
- Cancel/Discard removes all uncommitted selector and dependent state under existing form rules.

## Responsive and Viewport Contract

- Method remains subject to the existing connection-form active-row viewport projection.
- When Method owns focus, it and all three labels remain visible; a scrollbar, when needed, occupies only the established right-padding column.
- Selector rendering MUST NOT change outer panel geometry or scrollbar calculation.
- Resize across wide/stacked layouts preserves selected method, focus, drafts, errors, and logical offset.
- Below 40x12, the existing undersized surface replaces the form without accepting selector input; restoring a supported size reveals the unchanged selector state.
- Rendered rows remain bounded by reported width and height and use existing Unicode/display-width truncation rules for surrounding content.

## Help and Documentation Contract

- Contextual Actions while Method is focused MUST identify `Left/Right` as the method-change control.
- Form Help MUST state that Left/Right cycles through Agent, Key, and Password.
- Help MUST NOT advertise typing, deletion, paste, mouse input, or a separate activation key for Method.
- README MUST distinguish the closed selector from editable connection text fields and document conditional controls and retention behavior.

## Contract Test Matrix

| Area | Required cases |
|------|----------------|
| Initialization | New form 30 times; edit form 10 times per persisted method. |
| Cyclic movement | Left and Right from each option; 20 full cycles in each direction. |
| Focus traversal | Create/edit × each method × Tab, Shift+Tab, and F2. |
| Dependencies | All nine source-to-destination method transitions, including focus fallback. |
| Retention | Identity and Remember through repeated switching, Help, validation/save failure, and resize. |
| Input isolation | Three valid names, three invalid names, printable command keys, edit keys, modified arrows, Unicode, control-bearing paste, and malformed input. |
| Request projection | Create/edit × each final method × valid/invalid; identity clear and effective remember gating. |
| Accessibility | Color/no-color at 40x12, 79x24, and 80x24; exact focus and selected markers. |
| Responsive state | Existing twelve-size matrix plus 40x12→39x11→40x12 restoration. |

Every case verifies exactly one selection, no free-text mutation, no secret projection, and no regression to cancel/help/save/quit ownership.
