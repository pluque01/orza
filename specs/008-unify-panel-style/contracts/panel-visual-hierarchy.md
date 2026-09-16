# TUI Contract: Panel Visual Hierarchy

This contract defines the visible hierarchy shared by Orza's existing TUI surfaces. It changes presentation only. Existing keyboard input, focus ownership, actions, confirmations, persistence, SSH behavior, credential handling, error recovery, outer geometry, viewport state, and scrollbar policy remain unchanged unless this contract explicitly addresses rendering.

## Badge Contract

Every visible panel identifies its type with one controlled textual badge.

Plain and no-color form:

```text
[Connection]
```

Color form uses the same characters and text with a bold application-blue background and readable foreground. ANSI stripping MUST produce the plain form exactly.

- Badge labels are controlled English and MUST NOT contain target, catalog, diagnostic, or secret data.
- A title that already identifies the panel type becomes the badge; it MUST NOT be repeated in the body.
- A second content badge appears only when its type differs from the container type.
- Details retains its stable region badge and identifies the selected content separately.
- Focus, selection, invalid, warning, and primary-action markers remain textually independent from badge color.

### Browser Inventory

| Surface | Container badge | Distinct content badge |
|---------|-----------------|------------------------|
| Catalog tree | `Tree` | None |
| Details region | `Details` | `Root`, `Folder`, `Connection`, the active connection-form heading, or the active security-prompt heading |
| Actions region | `Actions` | None |

Region focus remains visible before the badge:

```text
[*] [Tree]
[ ] [Details]
[ ] [Actions]
```

### Modal Inventory

The existing `modalTitle` mapping remains the source of modal badge labels: `Create Folder`, `Edit Folder`, `Move`, `Delete`, `Connect`, `Unsaved Changes`, `Help`, `Operation Error`, and `SSH Failure`. An unknown modal kind uses the safe fallback `Panel`; an invalid payload under a known kind retains that kind's badge and displays the existing safe invalid-payload error.

- Exact body headings that merely repeat the modal badge MUST be removed.
- Questions such as `Connect to SSH target?`, state such as `SSH startup failed`, and actionable warning summaries remain because they add meaning beyond the type.
- Opening contextual Help replaces body content under the current modal badge without duplicating a `Help` heading inside the body.

## Structured Label Contract

Every controlled descriptive field label on an included surface is rendered without a trailing colon and with secondary emphasis in color mode. Values remain primary text unless existing focus, invalid, warning, or primary semantics take precedence.

Aligned plain-text example:

```text
[Connection]
Name      production
Path      /team/production
Endpoint  prod.example:2222
User      deploy
Method    key
```

- The longest controlled label in one logical group determines the shared value column.
- Labels and values are separated by at least one visible space.
- Label width uses terminal display cells, not bytes or rune count.
- Colons inside values, sentences, key combinations, and network endpoints are not removed.
- `Warning:` and `Error:` are status prefixes, not descriptive field labels; they retain their exact punctuation so existing actionable messages remain unchanged and recognizable without color.

## Narrow Field Contract

For each non-interactive identity group, aligned mode is used only when at least eight display cells remain for values after the maximum label width and separator. Otherwise every identity field stacks without omission:

```text
Name
  production
Path
  /team/production
```

- The value follows its label immediately on the next logical line.
- The value line uses visible indentation to preserve label/value hierarchy without color.
- No identity field is hidden to retain alignment.
- Long values retain the surface's existing wrap or safe-truncation rule.
- The layout decision uses local region/modal content width rather than only total terminal width.
- Interactive form fields do not use this mandatory stacking rule because the 40x12 Details region must retain a focused component and its validation error simultaneously.

## Details Contract

- The old `Kind` display field and `Kind: Connection` text MUST NOT render.
- Root, Folder, and Connection are rendered once as the Details content badge.
- Name, Path, Endpoint, User, Method, Identity, and Direct connections remain in their existing logical order when applicable.
- A default user remains visible as `(default)`.
- Identity appears only when applicable under existing detail rules.
- Direct child connection rows remain sorted and bounded; their names and endpoints adopt the same label/value hierarchy without changing child membership.
- Credential references and secret data MUST NOT enter badge, label, value, or overflow projection.

## Interactive Form Contract

- Connection and folder forms use the same colonless descriptive-label style and best available value-column calculation as static fields.
- Existing `>`, `!`, and `*` marker slots remain in the same semantic order and are not replaced by label color.
- At every supported size, an interactive field keeps its label and bounded component value on one compact row so the field's validation error can occupy the next visible row at 40x12.
- Text cursor, authentication selection brackets, checkbox state, field visibility, validation messages, Save priority, and focus traversal remain unchanged.
- The authentication selector's complete fixed option inventory retains its existing width priority and MUST NOT be truncated merely to align with a longer label.
- A layout change MUST preserve active-line/block indexes so the focused field and its error remain visible.
- Form titles identify the content inside Details or the modal and MUST NOT duplicate the container badge when semantically identical.

## Confirmation, Error, and Security Contract

- Confirmation and destructive-action targets retain complete wrapped disclosure where currently required.
- Removing label colons MUST NOT weaken target, ID, revision, scope, effect, previous-attempt, cause, stage, recommendation, or technical-detail visibility.
- Modal controls remain fixed or packed according to current priority and MUST NOT be displaced by badges or stacked fields.
- Conflict and operation states retain explicit `Warning` or `Error` text in addition to style.
- Trust prompts retain host, remote address, algorithm, fingerprints, changed/revoked warnings, reject default, and trust choices.
- Secret prompts retain only masked length; secret bytes and credential references never become display values.
- Host, endpoint, path, diagnostic, and other dynamic values pass through existing safe-text projection before styling.

## Actions and Help Contract

- Actions keeps the existing descriptor inventory, priorities, packing, and `Hidden actions — ? Help` overflow marker.
- Help keeps the existing contextual action inventory, scrolling, close controls, and controlled English.
- Styling MUST NOT add, remove, reorder, or imply any key, mouse action, click target, or activation behavior.
- Help and Actions titles use badges; their body content does not repeat those type names solely for decoration.

## Responsive and Viewport Contract

- Outer Tree, Details, Actions, and modal rectangles remain governed by the existing layout calculations.
- At 40x12 and above, non-interactive identity groups may switch independently between aligned and stacked layouts while interactive fields retain compact rows and semantic content priorities.
- Below 40x12, the current undersized surface replaces all panels and accepts only its existing Help/Quit behavior.
- Resize preserves focus, selection, form drafts, errors, modal payload, content owner, and logical viewport offset.
- Scrollbars remain informational and consume only the established right-padding cell.
- Projection MUST NOT inspect rendered colon-bearing prefixes to decide safety or disclosure behavior.
- Every visible line remains within its local width under ANSI styling, Unicode width variation, invalid UTF-8, bidi controls, and control-bearing user data.

## Color and Accessibility Contract

- Badge text and delimiters, region focus markers, row focus, selection, invalid, primary, warning, error, checkbox, and authentication-method selection all remain visible without color.
- `--no-color` and `NO_COLOR` emit no ANSI style sequences.
- For every semantic style primitive, stripping ANSI from the color rendering yields the corresponding no-color rendering.
- Muted labels MUST NOT hide text or replace semantic wording.
- No accessibility claim beyond these tested textual and terminal behaviors is introduced.

## Contract Test Matrix

| Area | Required cases |
|------|----------------|
| Badges | Tree, Details, Actions; Root/Folder/Connection content; every registered modal kind; connection/folder forms; trust and secret prompts; color/no-color equivalence. |
| Duplication | Help, Connect, Delete, operation error, SSH failure, and embedded content headings appear once per semantic level. |
| Labels | Every controlled descriptive field across Details, forms, move picker, confirmations, errors, conflicts, and prompts has no trailing colon; `Warning:` and `Error:` status prefixes remain unchanged. |
| Layout | Aligned value starts for wide identity groups; exact boundary around the eight-cell value floor; stacked read-only identity pairs below it; compact interactive field/error blocks at 40x12; no omitted identity fields. |
| Focus and errors | Form marker order, compact active block, field error, Save error, modal controls, and recovery actions remain visible in every supported layout. |
| Safety | Long Unicode and control-bearing values stay inert and bounded; confirmation targets wrap fully; credential and secret canaries never render. |
| Responsive state | Existing size matrix plus `40x12→39x11→40x12`, repeated 20 times, preserves state and recomputes local field layout. |
| Performance | At least 19/20 selection, focus, Help, confirmation, and resize samples complete with rendering in 100 ms on the existing 1,100-node fixture. |

Every contract case also verifies unchanged keyboard and action inventories, unchanged outer geometry, bounded output, and stable viewport/control ownership.
