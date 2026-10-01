# Quickstart: Validate Tree Connection Filtering

## Prerequisites

- Go 1.26.0 with the configured `go1.26.6` toolchain.
- Repository dependencies available in `vendor/`.
- An interactive ANSI/VT terminal for manual validation.

## Automated Validation

Run the focused TUI and integration coverage:

```bash
go test ./internal/tui ./tests/integration
```

Run the complete suite before merge:

```bash
go test ./...
```

The feature-specific tests should create nested folders and named connections, then prove the behaviors in [tree-search.md](./contracts/tree-search.md) and the state rules in [data-model.md](./data-model.md).

## Manual Validation

1. Launch Orza without a subcommand in an interactive terminal and load a catalog with nested folders and at least three connections.
2. Focus the Tree panel, press `/`, and enter a partial connection name. Confirm that only matching connections and their ancestor folders remain, including matches under initially collapsed folders.
3. Enter a valid regular expression matching multiple names. Confirm that every matching portion of every result name is distinguished and that folders themselves never become results.
4. Use Up Arrow, Down Arrow, Ctrl+P, and Ctrl+N. Confirm movement is limited to matching connections and stops at both ends.
5. Press Escape. Confirm the complete tree, prior selection, folder expansion, viewport position, and focus state are restored.
6. Start another search, select a matching connection, and press Enter. Confirm the natural tree returns with that connection focused, every folder in its path expanded, and no connection confirmation or session starts.
7. Enter an invalid expression and then a valid query with no matches. Confirm each gives visible feedback, leaves the catalog unchanged, and can be corrected or cancelled.
8. Repeat representative cases with color disabled and in a narrow terminal. Confirm the active result remains distinguishable, long lines remain bounded, and keyboard operation works throughout.
