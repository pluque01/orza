# TUI Contract: Árbol contextual y pegado

## Catalog Tree

The catalog screen renders one vertical preorder tree containing the root, folders, and connections.
Each child is on a later line than its parent and is indented by exactly two terminal cells per level.
Siblings place folders before connections and order each kind by binary, case-sensitive name and then
ascending ID. The root is always the first row, expanded, and initially selected. Other folders begin
collapsed. ASCII markers are fixed: `[/]` root, `[+]` collapsed folder, `[-]` expanded folder, `[ssh]`
connection, and `>` selected row.

The details panel always names the selected node's full path, ID, kind, and revision. Connection details
also show endpoint and authentication metadata already present in the catalog. Secret values are never
shown.

When the terminal is below 80x24, secondary details may be hidden, but the selected row, full path,
active action/confirmation, error, and keyboard cancel/back/exit remain visible and usable.

## Tree Keymap

| Key | Behavior |
|-----|----------|
| `Up` / `k` | Select previous visible row. |
| `Down` / `j` | Select next visible row. |
| `Home` / `g` | Select root/first visible row. |
| `End` / `G` | Select last visible row. |
| `Right` / `l` | Expand selected folder; if already expanded, select its first visible child. |
| `Left` / `h` | Collapse selected expanded folder; otherwise select its parent. |
| `Enter` / `Space` on folder | Toggle expansion without leaving catalog screen. |
| `c` on connection | Open target confirmation before the connection workflow. |
| `n` | New connection in selected folder, or sibling of selected connection. |
| `f` | New folder in selected folder, or sibling of selected connection. |
| `e` | Rename selected folder or edit selected connection; root rejects safely. |
| `m` | Move selected folder/connection; root rejects safely. |
| `d` | Delete selected folder/connection; root rejects safely. |
| `r` | Reload complete catalog tree. |
| `?` | Toggle help. |
| `q` / `Ctrl+C` | Exit from catalog screen according to existing application contract. |

Navigation does not wrap at the first/last row. A command invalid for the selected kind leaves
selection and catalog unchanged and shows a concise actionable message.

## Contextual Action Contract

- Folder selection plus `n`: prefill destination with that folder.
- Connection selection plus `n`: prefill destination with its parent folder.
- Folder selection plus `f`: create child folder there.
- Connection selection plus `f`: create sibling folder in its parent.
- Rename, move, delete, edit, and connect capture selected `NodeID` before starting async work.
- Destructive confirmation displays the captured full path and kind.
- Connection confirmation displays captured full path and `host:port`; acceptance sends the captured ID
  and expected revision, which the application service rechecks before network I/O. A mismatch rejects
  and reloads; cancel returns to the captured node.
- Requests include the captured revision where the existing application request supports it.
- Create requests include captured parent revision/path, and move requests include captured source and
  destination revision/path; repositories compare them atomically so ancestor path changes conflict.
- If the node is missing/stale, no alternate row is used. Show conflict, reload, and reconcile selection.
- Saving a create selects the returned ID and expands its ancestors.
- Canceling any form returns to the same node when it still exists and persists nothing.

## Reload And Selection

A reload keeps the last valid tree visible while loading. A successful response atomically replaces it.
Expansion survives for folder IDs still present. Selection resolution order is:

1. Pending created/moved ID, when supplied and present.
2. Previously selected ID.
3. Nearest surviving parent captured before the operation/reload.
4. Root.

If a selected descendant becomes hidden by collapsing an ancestor, selection moves to the collapsed
folder before rows are removed. Exactly one visible row remains selected.

Expansion and selection survive transitions that return from forms and confirmations during the same
TUI execution. An active SSH session terminates the TUI and therefore has no browser return state.

## Normal Text Input

The editable metadata inventory is connection `Name`, `Folder`, `Host`, `Port`, `User`, `Method`,
`Identity file`, and folder `Name`. Each accepts terminal-delivered bracketed paste through Bubble Tea
`PasteMsg`. Only the focused editable field receives it. `PasteStartMsg` and `PasteEndMsg` have no
application action. No field invokes an OS clipboard API; Bubbles' default paste key binding is disabled.

Fields support cursor movement and text selection with Shift+Left/Right/Home/End when the terminal
reports modifiers. Pasting or typing replaces the active selection. Selection remains visible without
depending only on color.

### Paste transaction

For each `PasteMsg`:

1. Capture value, cursor, and selection.
2. Remove CR, LF, NEL (`U+0085`), Line Separator (`U+2028`), and Paragraph Separator (`U+2029`).
3. If any other Unicode control character remains, reject the entire payload.
4. Build the candidate by replacing selection or inserting at cursor.
5. Reject without truncation if the candidate exceeds 4,096 Unicode runes, then apply immediate format constraints.
6. Commit value and cursor once, or preserve the exact captured state on rejection.

Empty/canceled paste is a no-op. A rejection displays `Paste rejected: unsupported control character`
or another generic field-safe reason and does not include payload content. Letters, escape sequences,
newlines, `q`, `d`, `Enter`, or `Ctrl+C` inside a paste are never dispatched as commands.

Normal form submission still runs the existing complete domain validation, so accepted pasted text has
the same eventual rules as manually typed text.

## Secret Input

Remembered-password, SSH-password, and private-key-passphrase prompts remain outside the Bubble Tea update loop through
`terminal.Terminal.ReadSecret`. The dedicated prompt:

- Activates raw/no-echo mode and bracketed paste.
- Displays one mask cell per Unicode rune and a non-color selection distinction; length/timing leakage is accepted.
- Supports Left/Right/Home/End, Shift selection, Backspace, Delete, Enter, Escape and Ctrl+C.
- Applies the same atomic paste transaction and never treats pasted content as keys.
- Submits only on an actual Enter key event.
- Cancels only on an actual Escape/Ctrl+C event or context cancellation.
- Has no history, clipboard API, trace logger, diagnostic payload, or unmasked render path.
- Rejects a candidate over 4,096 bytes atomically and never truncates it.
- Cancels/joins the reader and restores bracketed paste, cursor/style, terminal mode and newline before
  returning after submit, cancel, context cancellation, catchable SIGINT/SIGTERM, or error.
- Owns the result until cleanup completes, then transfers bytes temporarily to authentication; only an
  explicit `Remember password` selection may persist a copy in the operating-system credential store.

If terminal cleanup fails after submission, the prompt wipes its controlled result and returns a generic
error instead of returning a secret under uncertain terminal state.

## Terminal Capability Contract

Full paste isolation is supported only on a terminal emulator that honors bracketed paste and emits
VT-compatible keyboard events. If unsupported, the application never reads the system clipboard,
remains editable/cancelable, and makes no isolation guarantee for bytes delivered as ordinary typing.
Help and user documentation always state this limitation because an emulator silently ignoring the
enable sequence cannot be detected reliably.

On Windows, secure secret editing requires the process standard console input handle so reads can be
canceled safely. Unsupported redirected or alternate handles fail with an actionable, secret-free
terminal capability error. The pinned Ultraviolet reader must also be positively identified as its
native cancelable Windows reader; any silent fallback fails before capture/raw/output. Linux/macOS use
a terminal/PTY accepted by the existing interactive check. Cleanup joins the reader before restoration.

The process root owns signals. After the prompt restores its state, `os.Interrupt`/SIGINT exits with 130
and Unix SIGTERM exits with 143; signals never return control to a still-running TUI. Windows uses its
supported `os.Interrupt` path and context cancellation rather than claiming Unix-only SIGTERM behavior.

`NO_COLOR` changes styling only; markers, selection, errors and masks remain understandable.

## Error Contract

- Catalog load failure: retain last valid snapshot where possible, show retry with `r`.
- Revision mismatch/missing target: perform no mutation against another node; reload and explain stale target.
- Invalid paste: preserve field state exactly and show generic reason.
- Terminal without paste delivery: no state change; typing/cancel remain available.
- Reader or output failure: wipe controlled secret buffers, restore terminal, return recoverable prompt error.
- Cleanup failure: continue all remaining cleanup, combine internal errors, expose no input bytes.
