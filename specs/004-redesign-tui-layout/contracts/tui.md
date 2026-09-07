# TUI Contract: Responsive Multipanel Interface

> **Superseded overflow rules**: `specs/005-add-scrollbar-indicator/contracts/tui-scrollbar.md` controls all vertical overflow indicators and the Actions omission cue.

## Base Composition

Normal frames contain titled `Tree`, `Details`, and `Actions`. Details is read-only projection or connection
form. Zero/one centered panel may overlay the frame. Every frame is bounded by terminal display cells; color
only enhances textual state.

### Exhaustive Context Content

| Context | Required content |
|---|---|
| Tree root/folder row | `>` selection or spaces, two cells per depth, `[/]` root or `[+]/[-]` folder marker, name. |
| Tree connection row | `>` selection or spaces, two cells per depth, `[ssh]`, name. |
| Connection Details | `Connection`, name, full path, endpoint, user, auth method, optional identity reference; no secret. |
| Folder/root Details | `Folder`/`Root`, name, full path, direct count, direct connection name+endpoint rows or explicit empty state. |
| Connection form | Mode/target, Name, Folder, Host, Port, User, Method, conditional Identity/Remember, field errors, form error, Save. |
| Actions | Exact controls from the winning owner/state row: domain, form, modal, recovery, security or operation, plus applicable local navigation. |
| Centered panel | Exact body and controls in the Centered-Panel Matrix below. |

The table is exhaustive for required content. Secondary prose may be added only when space remains and cannot
change target, ordering, action inventory or reduced priorities.

## Display Modes

| Condition | Contract |
|---|---|
| Width >=80 and size >=40x12 | Wide: Tree left, Details right, Actions below. |
| Width <80 and size >=40x12 | Stacked: Tree, Details, Actions vertically. |
| Width <40 or height <12 | Undersized shell. |

80x24 shows selected row/field, detail identity and every context action together. Any non-undersized frame
with width <80 or height <24 is reduced; orientation still follows width. Thus 80x12 and 100x20 are wide
reduced. Reduced mode prioritizes active row/field, target, error and recovery/cancel/back/quit.

Geometry includes borders. Reserve one column between wide Tree/Details. Tree width is
`floor((terminalWidth-1)*0.40)` and Details receives the remainder. Actions outer height is exactly 5 rows in
every non-undersized mode; base panels receive the remainder. Stacked uses the
same Actions rule and splits remaining rows equally; an odd row goes to
the focused Tree or Details/form base region. When modal/security owns input, the preserved application
FocusOwner determines the recipient. Every rectangle is deterministic, non-negative and terminal-bounded.

Every normal region rectangle includes a one-cell border. Its title is embedded in the top border and consumes
no content row. Horizontal interior padding is one cell per side; there is no vertical padding. Interior width
is `max(0, outerWidth-4)` and interior height is `max(0, outerHeight-2)`. The one-column wide gutter belongs to
neither panel. Actions always spans terminal width below the base panels.

Actions descriptors use the priority order below, then the stated stable order within each priority, with one
ASCII space between descriptors. Packing is
greedy left-to-right: append the next complete descriptor iff separator+descriptor fits; otherwise start the
next row. A descriptor wider than the interior is truncated in-width with `…`. If fixed-height packing omits
descriptors after all priority-1/2 controls are packed, the final row available to priority 3-6 becomes
`↓ more — ? Help`; this marker never replaces priority 1/2. Actions is
not focusable/scrollable and Help exposes the complete scrollable list.

Actions classifies and packs content in this exact order:

1. Active `Loading:`/error/conflict status and captured target identity; these are content, not descriptors.
2. Recovery/safety descriptors in displayed-applicable order: `Esc Cancel`, `b Back`, `r Reload` or `r Retry`,
   `q Quit`, `? Help`. Every applicable priority-2 descriptor must remain visible at 40x12.
3. Primary completion descriptors: `Ctrl+S Save`; `y Confirm`; unsaved `s Save` then `d Discard`; move `Enter Move`.
4. Remaining domain descriptors in the exact root/folder/connection inventory order.
5. Focus/local navigation descriptors in their keyboard-row order.
6. Secondary hints/prose.

Within one priority, listed order is stable. A descriptor has only its canonical full label; compact aliases or
alternate labels are prohibited. A too-wide single descriptor truncates with `…`, preserving its key prefix.

Reduced priority in every region is: active selection/field/error; target; recovery/Cancel/Back/Quit; primary
action; remaining required content/actions; navigation; secondary prose. Only a later category may be omitted.

Undersized shell preserves selection, expansion, offsets, form values, focus, modal, conflict, operation and
intent. It shows required `40x12`, allows Help and safe Quit, and routes Quit through dirty/operation cleanup.
Help uses the existing modal state but fills the bounded undersized shell. Returning to >=40x12 reveals exact
preserved state.

## Overflow and Truncation

- Scrollable regions show `↑ more` in first content row for previous content and `↓ more` in last for next.
- Both appear when both directions exist. They are non-focusable.
- Every truncated line includes `…` within assigned display width.
- With one/two rows, active selection/field/error/recovery wins; a marker uses only remaining row.
- Markers never displace active/error/recovery/cancel/quit.
- Rules apply to Tree, Details, connection form, Actions/Help, move picker, confirmation and error payloads.
- Tree/Details/direct-connection names, paths, endpoints, editable field values, field labels and one
  `<key> <label>` descriptor are single-line and truncate by display-cell width with `…`. Actions wraps only
  between complete descriptors. Help/confirmation/error prose wraps at Unicode grapheme boundaries; vertical
  clipping uses directional markers.
- Destructive/connect confirmation identity is the exception: captured full path, endpoint and ID/revision wrap
  safely at grapheme boundaries and never truncate. Vertical overflow remains scrollable with directional markers.

## Tree and Details

- Tree preserves preorder, folder-first/name/ID order, two-cell indentation, expansion and markers `>`,
  `[/]`, `[+]`, `[-]`, `[ssh]`.
- Selection stays visible; missing selection after reload falls to nearest existing ancestor/root.
- Connection Details order: name/type, full path, `host:port`, user, auth method, identity reference.
- Folder/root Details: name, path, direct count, immediate connections only; explicit English empty state.
- `Tab`/`Shift+Tab` alternate Tree/Details. Focused Details uses `Up/k`, `Down/j`, `Home/g`, `End/G`; navigation
  scrolls only and never mutates selection/data.
- Direct connections use canonical name then ID order.
- Secrets and raw errors never appear.

## Normal Action Inventory

Actions, Help and dispatch consume exactly one descriptor inventory:

| Context | Exact object keys |
|---|---|
| Root | `n` New connection, `f` New folder, `r` Reload, `?` Help, `q` Quit |
| Folder | `n`, `f`, `e` Edit, `m` Move, `d` Delete, `r`, `?`, `q` |
| Connection | `c` Connect, `n`, `f`, `e`, `m`, `d`, `r`, `?`, `q` |

`Ctrl+C` aliases `q`; `n/f` on connection use parent. Focus/movement/expansion/scroll controls are shown in
addition. Keys for unavailable actions are inert.

## Closed Keyboard Matrix

Input uses the first active row by Focus and Input Priority. Command keys absent from that row are inert;
printable text, cursor/edit/delete/selection and bracketed-paste events remain local to the focused editable
field and never dispatch as application commands. In editable text, `F1` opens Help, `F2` is Previous fallback,
`Ctrl+C` safely quits, and printable `p`, `q` and `?` are always text.

| Owner/state | Exact local keys |
|---|---|
| Tree | `Up/k`, `Down/j`, `Home/g`, `End/G`, `Left/h`, `Right/l`, `Enter/Space` toggle only for an expandable folder, `Tab/Shift+Tab` Details, current object inventory. |
| Details | `Up/k`, `Down/j`, `Home/g`, `End/G`, `Tab/Shift+Tab` Tree, current object inventory. |
| Connection form | Local text edit/paste; `Tab` Next, `Shift+Tab/F2` Previous, `Ctrl+S` Save, `Enter` activate focused control, `Esc` Cancel, `F1` Help, `Ctrl+C` safe Quit. On non-text controls, `?`/`q` also Help/Quit. |
| Folder form | Local text edit/paste; `Tab` Next, `Shift+Tab/F2` Previous, `Ctrl+S/Enter` Save, `Esc` Cancel, `F1` inline Help, `Ctrl+C` safe Quit. On non-text controls, `?`/`q` also Help/Quit. |
| Move picker | `Up/k`, `Down/j`, `Home/g`, `End/G`, `Enter` Move, `Esc` Cancel, `?` inline Help. |
| Confirmation | `Up/k`, `Down/j`, `Home/g`, `End/G` scroll wrapped identity/prose, `y` explicit Confirm, `Enter/Esc` Cancel, `?` inline Help. |
| Unsaved changes | `s` Save, `d` Discard, `Esc` Cancel, `?` inline Help. |
| Help | `Up/k`, `Down/j`, `Home/g`, `End/G`, `?/Esc` Close. |
| Operation error | `r` Reload for missing/conflict, otherwise `r` Retry; `b/Esc` Back/Close, `?` inline Help, `q/Ctrl+C` Quit. |
| SSH failure idle | `d` Detail only when technical detail exists, `r` Retry, `e` Edit, `b/Esc` Back/Close, `?` inline Help, `q/Ctrl+C` Quit. |
| SSH failure missing/conflict | `d` Detail only when present, `r` Reload, `b/Esc` Back/Close, `?` inline Help, `q/Ctrl+C` Quit; no Edit. |
| SSH failure resolving | `d` Detail only when present, `b/Esc` Back/Close, `?` inline Help, `q/Ctrl+C` Quit; no Retry/Reload/Edit. |
| SSH retry confirmation inline | `y` Confirm starts SSH for freshly resolved ID/revision; `Enter/Esc` Cancel returns to unchanged SSH diagnostic; `?` inline Help. |
| Conflict | `r` Reload, `b` Back, `Esc` compact warning, `?` Help, `q/Ctrl+C` Quit. |
| Loading | `Esc` Cancel, `?` Help, `q/Ctrl+C` Quit; if Help visible, Help-local scroll keys only. |
| Trust security input | `Up/k`, `Down/j`, `Home/g`, `End/G` local scroll, explicit `y` Accept when allowed, `Esc` Cancel, `q/Ctrl+C` safe Quit; resize never accepts. |
| Secret security input | Masked local text/edit/paste including printable `q/?`; `Enter` Submit, `Esc` Cancel, `F1` Help, `Ctrl+C` safe Quit; raw text never reaches Actions/model. |

If Shift+Tab cannot be distinguished, Help/Actions exposes unmodified `F2` as Previous for form traversal. Shared
keys resolve only in the winning row: `d` is Delete in normal object inventory, Discard in unsaved changes and
Detail in SSH failure; `r` follows the displayed Reload/Retry label; `?` is inline when another modal owns focus.

## Connection Form

- Create/edit remains in Details and never allocates centered panel.
- First editable field starts focused; Tab/Shift+Tab traverse every visible control deterministically.
- Focus/invalid/save have separate textual markers. Viewport keeps field and error visible through resize.
- Tree remains visible but inert. Save selects result; cancel restores prior tree context.
- Error/conflict preserves values/focus/target. Dirty Quit opens Save/Discard/Cancel modal.
- Save+Quit exits only after commit, reconciliation and cleanup; failure/conflict never exits.

## Closed Centered-Panel Inventory

Exactly these kinds may use generic centered panel:

1. Folder create.
2. Folder edit.
3. Move destination.
4. Delete connection.
5. Delete folder.
6. Connect confirmation.
7. Unsaved changes.
8. Help.
9. Recoverable operation error.
10. SSH failure.

Panel owns focus, identifies action/target, blocks background keys, fits viewport and exposes visible
cancel/close. Destructive/connect requires explicit acceptance; Enter/Escape never silently accepts. Errors
and conflicts embed in active panel. If Help is requested while modal owns an operation, `helpVisible` toggles
inline bounded content in that panel; `?` toggles it and `Esc` from inline Help restores the previously focused
payload control without closing/replacing the modal. No other generic modal kind is valid. Delete connection
and delete folder are separate kinds over the same renderer because their captured target/scope differs.
SSH failure may enter an inline retry-confirmation subphase within the same kind/owner after `r` resolves the
captured ID; this is not a second/replacement modal. `y` starts SSH for the freshly resolved ID/revision;
`Enter/Esc` cancels back to unchanged diagnostics with zero network.

### Centered-Panel Matrix

| Kind | Required body | Terminal paths and result |
|---|---|---|
| Folder create | Action, captured destination path/ID, Name field/error | Save creates/selects folder; Cancel restores opener. |
| Folder edit | Action, captured path/ID/revision, Name field/error | Save renames/selects same ID; Cancel restores opener. |
| Move destination | Action, source path/ID/revision, valid destination list | Move selects moved ID; Cancel restores opener. |
| Delete connection | Action/effect, full path, endpoint, remembered-secret effect | Confirm deletes and selects nearest ancestor; Cancel restores opener. |
| Delete folder | Recursive effect, full path, folder/connection/credential counts | Confirm deletes and selects nearest ancestor; Cancel restores opener. |
| Connect confirmation | Full path, endpoint, ID/revision | Confirm starts SSH owner; Cancel restores opener with zero network. |
| Unsaved changes | Dirty target, Save/Discard/Cancel choices, save error inline | Save exits after commit/cleanup; Discard exits; Cancel restores form. |
| Help | Applicable key map and meanings | Scroll; Close restores opener. |
| Operation error | Action, target, safe cause/category, recommendation | Missing/conflict: Reload; otherwise Retry; Back/Close via `b/Esc`; owner data retained on failure. |
| SSH failure | Attempt path/endpoint/ID, category/stage/recommendation, optional detail; fresh target confirmation after successful resolve | Idle: Retry/Edit; missing/conflict: Reload; resolving: neither; Detail iff nonempty; Back/Close via `b/Esc`; `y` confirms inline retry and `Enter/Esc` returns to diagnostic. |

Each enabled terminal path is normative; no representative kind substitutes for another in acceptance.

## Conflict Contract

- Missing navigation selection after reload chooses nearest existing ancestor/root and refreshes Details/Actions.
- Navigation same-ID revision change keeps the ID selected, refreshes Details/Actions from the new snapshot and
  reports the change without entering ConflictState or exposing recovery controls.
- Form/modal/operation missing or revision-changed target preserves CapturedTarget, values and focus; blocks
  persistence/network and shows compact conflict banner.
- `r` Reload refreshes and rechecks same ID without retargeting or value loss.
- `b` Back abandons interaction and selects nearest existing ancestor/root.
- `Esc` Cancel warning hides expanded detail but leaves banner, blocked state and recovery controls.
- No row index may authorize target resolution.

## Async Operation Contract

- Exactly one operation owner: initial load, reload, save or SSH start.
- Last valid frame stays visible; initial load uses empty shell.
- Actions shows exact `Loading: <action> — <target>` using captured path/endpoint or `catalog`.
- While running, operation filtering allows only `Esc` Cancel, `?` Help and `q`/`Ctrl+C` Quit for application
  surfaces; active security input is dispatched before this filter and uses its own closed row.
- If Help is visible, prompt-local scroll/navigation controls are additionally dispatchable but never appear as
  domain actions and cannot mutate catalog/network/operation.
- Cancel/Quit requests cancellation and waits reader/network/terminal cleanup.
- Commit points: accepted snapshot; post-commit save result; active SSH session with nonzero `StartedAt`.
- Before commit, cancellation restores. After commit, confirmed result applies before restore/quit.
- Only matching current operation ID may update state/focus/content; stale/duplicate results are ignored.
- While phase is running/cancel_requested/cleaning, operation keys have precedence and no conflict is
  dispatchable. A conflict owner result completes cleanup, clears operation ownership, then activates the
  conflict banner and `r`/`b`/`Esc`/`?`/`q` inventory on the next frame.
- Same-ID/revision is checked before dispatch. Pre-commit conflict performs no I/O commit. Post-commit result is
  reconciled before cleanup and is not reclassified as a pre-commit conflict; stale/non-owner completion is inert.

## Errors, Language, and Safe Projection

- Application-controlled titles/actions/help/status/validation/confirmation/errors are English and use the
  canonical vocabulary in this contract and existing CLI.
- User names/paths/hosts/users remain byte-identical in model/persistence and are not translated.
- Presentation renders CR/LF, ANSI/control/bidi and invalid sequences inert; printable Unicode otherwise
  remains unchanged except in-width `…` truncation.
- Validation stays by field; modal operation error remains in owner; safe diagnostics never format raw cause.
- Connection save/persistence failure is a form-level inline error above Save and never opens operation_error;
  operation_error is only for an owner without an existing form/modal error surface.
- Output is bounded in wide, stacked, reduced, undersized and no-color modes.

## Host Trust and SSH Lifecycle

- Existing verifier and specialized trust prompt remain unchanged in authority and outside generic modal enum.
- Trust and secret prompts are specialized `SecurityInputOwner` states outside ModalState and preempt operation
  filtering without replacing the preserved application FocusOwner. Trust accepts only an explicit decision;
  secret input keeps existing masking/lifecycle and never enters presentation state.
- In undersized mode, security input remains pending and hidden behind the common minimum/Help/safe Quit shell;
  acceptance or secret entry is impossible until resize restores at least 40x12. Resize itself never accepts.
- Known matching key proceeds. Unknown/changed key shows target, fingerprint/status and requires explicit decision.
- Cancel performs no SSH session start and does not persist trust.
- Reload, retry, resize, cancellation and stale result cannot auto-accept or bypass trust decision.
- Session cancellation, timeout, auth/network failure, process signal and terminal cleanup retain existing
  deterministic behavior and exit codes.
- Remote stdout/stderr streams directly to terminal writers and never enters Model, viewport, modal, logs or
  diagnostic state. A 128 MiB two-stream test must retain no more than 1 MiB in the TUI; controlled technical
  diagnostics are at most 256 visible characters and contain no remote output, raw cause or secret.
- Timeout/network interruption before active-session commit produces recoverable SSH failure. After nonzero
  `StartedAt`, it preserves confirmed session state and ends with transport outcome. Both close session,
  transport, auth and resize forwarding exactly once, restore terminal before return/exit, clear operation owner
  and reject late results.

## Focus and Input Priority

1. Specialized trust/secret security input when active and not undersized.
2. Generic centered panel.
3. Connection form.
4. Focused Details.
5. Focused Tree.

Undersized filtering applies first; then security input dispatch; then operation/conflict filtering; then
application surface dispatch. Resize never reconstructs state.

Domain actions, recovery actions and navigation controls are separate inventories. Recovery replaces domain
actions while blocked; navigation controls augment only the focused region.

## Connection Form Order

Forward order is Name, Folder, Host, Port, User, Method, conditional Identity file, conditional Remember
password, Save; Shift+Tab is the exact reverse. Hidden conditional controls are skipped. If Method hides the
focused conditional control, focus returns to Method.

## No-Color and Accessibility

Active region, selected row, focused/invalid field, primary action, warning, failure, overflow and node kind
all have textual semantics. `NO_COLOR` and `--no-color` emit no ANSI styling. Every workflow has keyboard
cancel/back/exit and no meaning depends only on color.

Exact no-color cues: active title `[*]`, inactive title `[ ]`, then item marker slots in order focus `>`,
validity `!`, priority `*`; absent slots are spaces, so a focused invalid primary control is `>!*`. `Warning:`
and `Error:` prefix messages. Region borders use visible ASCII/Unicode line
characters with one interior padding cell and at least one separating cell. Existing color roles are title ANSI
12 bold, selected/focus ANSI 14 bold, muted ANSI 8, status ANSI 10, warning ANSI 11 bold and failure ANSI 9 bold;
no-color removes these tokens but not cues.

## Terminal Compatibility

Supported environments are VT-capable Linux, macOS and Windows terminals reporting printable keys, arrows,
Tab, Escape, F1/F2 and Ctrl+C. Color absence uses no-color cues. Indistinguishable Shift+Tab uses visible `F2 Previous`.
Missing required VT capability without safe fallback produces an actionable startup error after restoration.
Mouse navigation/activation is unspecified and never required for conformance.

## Conformance Matrix

- Sizes: Cartesian `{40,60,79,80,100,160} × {12,24}` repeated 20 times per state, plus
  `80x12→80x24→100x20→100x30` wide-short transitions.
- Undersized: 40x12→39x11→40x12 repeated 20 times for navigation/form/confirmation/operation.
- Navigation concurrency: missing uses ancestor fallback; same-ID revision change refreshes passively, 20 each.
- Conflict: missing/revision-changed × form/modal/operation × applicable `r`/`b`/`Esc`, 20 each.
- Operation: four kinds × success/failure/cancel/quit/stale/post-cancel/confirmed-before-cleanup, 20 each.
- Operation-to-conflict: each operation kind that can conflict completes cleanup, releases owner, then exposes
  conflict controls; 20 runs per applicable kind/conflict.
- Security input trust unknown/changed: accept/cancel × normal/reduced/undersized-suspended/recovered ×
  operation/cancellation/terminal-failure, 20 each. Secret password/passphrase: input/submit/cancel over the same
  applicable dimensions, 20 each. Undersized permits no accept/input/submit and only tests suspension/recovery.
- Stream bounds: 64 MiB stdout + 64 MiB stderr × success/network interruption, 20 each; ≤1 MiB retained remote
  content and ≤256 visible-character controlled diagnostic detail.
- Transport cleanup: timeout/network interruption × pre-active/active × applicable lifecycle stage, 20 each;
  exactly-once close/join, terminal restoration, zero operation owner and stale-result rejection.
- Modal: 20 runs per ten kinds; create/edit connection 20 each proving Details ownership.
- Language/safe text: all controlled corpus strings plus printable/control/Unicode user corpus.
- Render-history memory, performance and usability use exact SC-017, SC-011 and SC-008 protocols.

## Compatibility

Catalog semantics, CAS, paste limits/isolation, secret masking/cleanup, CLI output, host verification, SSH retry,
terminal restoration and process exits remain unchanged. No UI state persists between runs.
