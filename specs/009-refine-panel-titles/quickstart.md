# Quickstart: Validate Refined Panel Titles

## Prerequisites

- Go 1.26.6 toolchain available.
- Run commands from the repository root.
- Use the existing TUI test fixtures; no SSH server, credential, or catalog migration is required.

## Automated Validation

Run focused renderer coverage:

```sh
go test ./internal/tui
```

Run the complete suite:

```sh
go test ./...
```

Run TUI race coverage and static checks before merge:

```sh
go test -race ./internal/tui
go vet ./...
go build ./...
```

Expected outcomes:

- Structural title assertions render `[Tree]`, `[Details]`, region titles, and modal titles without a color background.
- Context assertions render `Connection`, `Folder`, and `Root` without brackets; ANSI-stripped color output matches no-color output.
- Palette tests verify the context-label foreground/background contrast threshold.
- Layout tests preserve focus markers, panel bounds, and responsive behavior.

## Manual TUI Validation

1. Start the existing TUI using the repository's documented local command.
2. Verify `[Tree]` and `[Details]` have no colored background while preserving focus markers.
3. Select a connection and a folder; verify their labels read `Connection` and `Folder`, have no brackets, and remain legible against their new background.
4. Open existing Actions, Help, confirmation, and error modals; verify their structural titles remain bracketed without background styling.
5. Run in no-color mode; verify titles and context types remain identifiable from text alone.
6. Resize within supported dimensions; verify titles and context labels stay inside panel bounds and existing focus, error, and action-priority cues remain visible.

See [the presentation contract](contracts/title-context-presentation.md) and [the transient model](data-model.md) for expected semantics.
