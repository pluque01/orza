# TUI Contract: SSH Startup Diagnostics

## Failure modal

Every pre-active failure displays, in priority order:

1. Safe summary and stable category.
2. Captured path and endpoint when available.
3. Blocking stage.
4. Category-specific recommendation.
5. Recovery controls.
6. Optional technical detail only when expanded.

Detail starts hidden for each new failed attempt. `d` toggles it without I/O or catalog mutation. Category,
target, stage, recommendation, and recovery controls remain visible while detail is expanded.

## Keymap

| Key | Behavior |
|---|---|
| `d` | Show/hide safe technical detail. |
| `r` | Resolve captured ID and open fresh connection confirmation; never start SSH directly. |
| `e` | Resolve captured ID and edit its current version. |
| `Esc` | Close failure and return/select captured node when present. |
| `q` / `Ctrl+C` | Exit according to existing stable-modal behavior. |

## Retry

1. Keep immutable failed ID/revision/path/endpoint.
2. Resolve current connection by captured ID with no SSH network I/O.
3. If missing/conflicting, retain old target in recovery context and offer back/reload.
4. If found, show fresh path/endpoint/revision confirmation.
5. If changed, show previous and current public targets.
6. Only `y` starts SSH using current ID plus expected revision.
7. Canceling confirmation returns without network or persistence.

## Edit and back

Edit opens the current catalog record resolved by captured ID, never stale failed values or current browser
selection. Back selects captured ID if still visible; otherwise it returns to a stable browser and reports
that the target no longer exists.

## Cancellation and lifecycle

Explicit cancellation handled by the application and returning control is category `canceled`; deadline is
`timeout`. Process-terminating signals preserve exit 130/143 and need not display a cancellation modal. A
nonzero remote status or failure after `StartedAt` is a session result and never opens this startup modal.

## Narrow/no-color

At 80x24 all mandatory fields and controls are visible. Reduced mode prioritizes category, target,
stage/recommendation, controls, then detail. ASCII labels and keys convey all state without color. Optional
detail never displaces recovery controls.

## Safety

The modal consumes only the safe projection. It never formats the wrapped cause. Resize, toggle, retry,
edit and back preserve this invariant and never expose a canary from error chains.
