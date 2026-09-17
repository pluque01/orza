# Quickstart: Validate Plain Panel Labels

## Prerequisites

- Go 1.26.6 through the repository Nix development shell.
- Run commands from the repository root.

## Automated Validation

```sh
nix develop -c go test ./internal/tui
nix develop -c go test ./...
nix develop -c go test -race ./internal/tui
nix develop -c go vet ./...
nix develop -c go build ./...
```

Expected outcomes:

- Structural titles use plain text after `[*]` or `[ ]`.
- Context labels are bold with no foreground or background color.
- ANSI-stripped color output matches no-color text.
- Existing bounds, focus, modal, and safe-text tests pass.

## Manual Validation

1. Start the TUI with and without color.
2. Verify `[*] Tree` and `[ ] Details` rather than bracketed titles.
3. Select a connection and folder; verify `Connection` and `Folder` are unbracketed, bold when supported, and have no background.
4. Open Help, confirmation, and error modals; verify their titles are unbracketed.
5. Resize within supported terminal sizes and verify focus markers and context remain visible.
