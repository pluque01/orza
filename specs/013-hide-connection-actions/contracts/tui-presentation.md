# TUI Presentation Contract

## Scope

This contract defines the user-visible control presentation for the existing interactive TUI. It does not add a public API or command.

| State | Actions Panel | Lower Control Region | Action Legend and Status |
|---|---|---|---|
| Catalog browsing | Absent | Existing browser legend area | Applicable browser legend remains visible |
| Form editing | Absent | Fixed existing region | Applicable form legend remains visible |
| Connection startup | Absent | Fixed existing region | Startup status and applicable cancel/quit controls remain visible |
| Host identity verification | Absent | Fixed existing region behind the modal | Trust modal controls remain visible |
| Conflict, recovery, and error | Absent | Fixed existing region | Applicable status and recovery legend remains visible |
| Minimum supported terminal size | Absent | Existing minimum-size layout | Readable action legend and safe controls remain visible where currently supported |

## Invariants

- No rendered TUI state displays an `Actions` panel title or border.
- Removing the panel does not redistribute its fixed lower-region space.
- Visible key labels remain consistent with the actions the user can invoke.
- Cancellation and quit behavior are unchanged.
