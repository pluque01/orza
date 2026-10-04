# TUI Contract: Permanent Tunnels Panel

Preserve the current catalog, palette, borders, text emphasis, viewport markers, and action legend. Add a dedicated Tunnels panel, not an Actions panel. UI copy remains English and keyboard-only.

## Browser Layout

At 80x24, a proposed exact baseline allocates the following rectangles, including borders:

| Region | x | y | Width | Height |
|---|---|---|---|---|
| Tree | 0 | 0 | 31 | 21 |
| Details | 32 | 0 | 48 | 11 |
| Tunnels | 32 | 11 | 48 | 10 |
| Borderless legend | 0 | 21 | 80 | 3 |

At 40x12: Tree `(0,0,40,3)`, Details `(0,3,40,3)`, Tunnels `(0,6,40,3)`, legend `(0,9,40,3)`. Each panel retains one content row plus its borders/title. At intermediate narrow sizes keep the same stacked topology and give extra rows to the focused panel while preserving at least one content row in every region. At wide sizes preserve the left Tree/right Details+Tunnels topology and distribute additional space proportionally, favoring the active region.

Below 40x12, retain existing undersized notice with Help/safe Quit; no trust approval or secret submission. Modal/security overlays may preempt browser content according to existing ownership rules. A healthy shell temporarily owns the terminal instead of drawing browser panels. These are the only existing overlay/lifecycle exceptions to normal-browser permanent visibility.

The current fixed five-row bottom allocation must change for this feature. The legend has at most three rows in normal browser layouts; prioritize focus/navigation, primary target action, Help, and safe Quit. Remaining actions remain discoverable in contextual Help. No Actions title/border is rendered. Detailed forms/inspection can use scrollable existing overlays when one-row browser content is insufficient.

## Panel Content and Selection

Title includes `Tunnels` and active count, which is independent of catalog filtering and selection. List entries in creation order across hosts in this TUI session. Show selected state/host/mode first at narrow widths; full requested/bound endpoints and warnings are accessible through inspection. Stopped/Failed entries remain inspectable until dismissed or bounded terminal-state eviction.

Empty state says `No tunnels` with contextual guidance to select a connection and press `p`. Overflow uses the existing proportional scrollbar and keyboard-selected-row visibility. Tunnel selection is a session TunnelID, not the catalog selected ID; switching either selection must not change the other.

Remote inspection shows `Requested listener`, not verified/bound address, and a prominent textual scope warning. Dynamic inspection explains `SOCKS5 proxy` and shows its local endpoint, with no fixed destination. Color adds emphasis only; all states and warnings retain text in no-color mode.

## Keyboard Contract

| Context | Key | Behavior |
|---|---|---|
| Browser | Tab / Shift-Tab or F2 | Cycle Tree -> Details -> Tunnels; reverse for previous |
| Browser catalog focus | p | Open forwarding draft for selected connection; inert on folders/root |
| Browser | t | Focus Tunnels without changing either selection |
| Tunnels focus | Up/Down or j/k | Select entries without wrapping; keep selected row visible |
| Tunnels focus | Enter | Inspect complete values, warnings, current safe diagnostic |
| Tunnels focus | s | Confirm stop for a Starting/Active selected tunnel; inert if no live target |
| Tunnels focus | r | Retry selected Stopped/Failed tunnel through current-record draft/review and confirmation |
| Tunnels focus | d | Dismiss selected Stopped/Failed entry; inert for live tunnels |
| Tunnels focus | Esc | Return to Tree focus without stopping anything |
| Browser | ? / F1 | Contextual Help |
| Browser | q / Ctrl-C | Safe exit request; list live tunnels and confirm or cancel |

Catalog `e`, `m`, `d`, `x`, `c`, `f`, and `n` mutations/connections are not dispatched while Tunnels owns focus. Printable keys belong to editable fields/search when those own input; no forwarding shortcut may activate through text paste. Modals/security input remain the sole input owner while open.

Inspection exposes the same applicable stop/retry/dismiss controls and Esc Back. Stop/exit confirmations identify captured targets and start on Cancel. Starting cancellation closes only that attempt. Exit includes Starting and Stopping as well as Active resources; handled termination bypasses confirmation but still cleans up.

## Creation Flow

1. Select saved connection and press `p`; a Details draft shows the captured host and Local/Remote/Dynamic choices, with Local selected.
2. Show `Listen address` default `127.0.0.1`, `Listen port`, and mode-appropriate `Destination host`/`Destination port`. Explain listening and destination-reaching machines in plain language. Dynamic hides fixed destination fields and explains proxy use.
3. Tab/previous traversal, F1 Help, Esc Cancel, and `Ctrl-S Start` follow existing form patterns; Ctrl-S starts validation, not persistent saving. Field errors retain draft values. Left/Right selects mode when its selector owns focus.
4. Non-loopback choice requires separate exposure consent, reset by mode/listener/target changes. Dynamic warning mentions unauthenticated proxy access.
5. Show captured path/SSH endpoint/mode/listener/destination summary with cancel-default target confirmation, check current revision before network activity, then perform existing trust/secret decisions.
6. Starting appears in Tunnels. Successful activation ends input-owning startup without canceling the runtime; browsing resumes with the new tunnel selected in Tunnels. Failure preserves settings for edit/retry and produces a safe stable error view.

One foreground startup/security interaction at a time; active tunnels continue throughout. Esc from a changed draft offers Discard/Cancel. No saved-profile control is shown. An error/modal closing restores the prior catalog and tunnel selections/focus.

## Shell and Quit

Opening `c` from catalog focus keeps all tunnels running using separate transports. Keep `tea.Exec` terminal release/restore. While the shell runs, UI events may be delayed but forwarding and cleanup are not. On normal/nonzero shell completion or shell-only transport failure, restore the browser, reconcile tunnel snapshots, and show a safe outcome notice. Do not return a shell status as the eventual result of unrelated TUI exit. Root termination/cancellation instead stops everything and exits after restoration/joining.

Acceptance includes traffic during the shell, one tunnel failing while the shell runs, restored focus after return, no-color/resize state preservation, exact focus-scoped key dispatch, and complete inspection at the minimum size.
