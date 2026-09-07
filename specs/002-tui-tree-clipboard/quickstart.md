# Quickstart: Árbol contextual y pegado en TUI

## Prerequisites

- Go 1.26.5 or `nix develop`.
- An interactive terminal with VT sequences and bracketed paste enabled for full paste tests.
- Native Windows/macOS runners for console and platform restoration checks.

Use an isolated data directory so manual scenarios do not touch a real catalog:

```sh
export XDG_DATA_HOME="$(mktemp -d)"
nix develop
```

On PowerShell, set an isolated supported application data directory according to the existing Windows
configuration instructions before launching the binary.

## Automated Verification

```sh
go fmt ./...
go test ./...
go test -race ./...
go vet ./...
nix flake check
```

Run targeted input/tree tests and the scale benchmark during implementation:

```sh
go test ./internal/tui ./internal/terminal -run 'Tree|Paste|Secret|Selection|Restore|Signal|Confirm'
go test ./internal/tui -run '^TestTreePerformanceAcceptance$' -count 1 -v
go test ./internal/tui -run '^$' -bench '^BenchmarkTree(Refresh|Expand|Collapse)$' -benchtime=20x -count=1
```

Cross-builds verify platform compilation but do not replace native tests:

```sh
nix build '.#windows-amd64'
nix build '.#windows-arm64'
```

Run `staticcheck` and `govulncheck` through the project's Nix checks or directly when available.

## Seed A Nested Catalog

```sh
go run ./cmd/orza folder create /work
go run ./cmd/orza folder create /work/production
go run ./cmd/orza folder create /work/production/eu
go run ./cmd/orza connection create /work/production/eu/bastion \
  --host bastion.example.com --port 22 --user deploy --auth agent
go run ./cmd/orza connection create /work/production/database \
  --host db.example.com --port 22 --user dba --auth agent
go run ./cmd/orza
```

## Manual Tree Scenarios

1. Confirm `/` is first, expanded and selected; other folders start collapsed. Confirm siblings use
   folders-before-connections and binary name/ID order, two-cell indentation, and the specified ASCII markers.
2. Use only arrows or `h/j/k/l` to expand `/work/production`, enter `eu`, return to root and collapse it.
3. Select `/work/production/eu`, press `n`, create `api`, and confirm destination is that folder, all
   ancestors are expanded, and the new connection is selected.
4. Select `bastion`, press `n`, and confirm the proposed destination is `/work/production/eu`.
5. Rename and move a selected node; confirm selection follows the same ID. Delete it and confirm the
   nearest surviving parent is selected.
6. Leave the TUI open, mutate/delete the selected node with another process, press `r`, and confirm no
   action targets a replacement row and fallback selection is deterministic.
7. Resize below 80x24 and confirm selected row, path, errors and cancel/exit controls remain usable.
8. Press `f` over root, folder and connection and confirm the destination contract for each selection.
9. Press `c` over a connection and confirm its path and endpoint appear before any network session starts;
   cancel and confirm the same node remains selected.
10. While confirmation is open, edit the connection from another process; accept and confirm the expected
    revision rejects before network I/O, reloads, and never connects to the changed endpoint.

## Manual Paste Scenarios

For connection `Name`, `Folder`, `Host`, `Port`, `User`, `Method`, `Identity file`, and folder `Name`:

1. Type prefix/suffix, move the cursor between them, use terminal paste (`Ctrl+Shift+V`, `Cmd+V`, or
   terminal equivalent), and confirm insertion occurs at the cursor.
2. Select part of the value with Shift+arrows and paste ASCII, accented text and variable-width Unicode;
   confirm only the selection is replaced.
3. Paste 1.001 and 4.096 runes and confirm acceptance under normal validation; paste a 4.097-rune
   candidate and confirm atomic rejection without changing value, cursor or selection.
4. Paste `qndec` and confirm no quit, create, delete, save, cancel or connect action runs.
5. Paste separators CR, LF, NEL, `U+2028` and `U+2029` and confirm their removal without submission.
6. Paste a payload containing tab, NUL, Escape or Ctrl+C and confirm the complete payload is rejected,
   the prior value/cursor/selection is unchanged, and the error contains none of the payload.
7. Try an empty/canceled paste and a terminal with bracketed paste disabled; confirm help/documentation
   state the reduced guarantee, the app never reads the OS clipboard, and typing/cancel remain usable.

## Manual Secret Scenario

Use a disposable test credential, never a real password:

1. Exercise remembered-password, SSH-password and private-key-passphrase prompts and paste a canary such
   as `SECRET_CANARY_002` between typed text.
2. Confirm only masks appear, selection replacement works, and a pasted newline does not submit.
3. Confirm pasted `Ctrl+C`, Escape, tab or other controls reject atomically and do not cancel.
4. Confirm physical Enter submits and physical Escape/Ctrl+C cancels.
5. Search captured output, application logs and test diagnostics for the canary; it must not appear.
6. Cancel during input and inject reader/output failures in automated tests; confirm echo, cursor,
   bracketed paste and raw mode are restored.
7. Enter 4.096 secret bytes and then exceed the limit; confirm the latter is rejected atomically.
8. Send `os.Interrupt`/SIGINT and Unix SIGTERM while the prompt owns the terminal; confirm restoration
   precedes process exit 130/143. On Windows validate `os.Interrupt` plus context cancellation. Document
   that `SIGKILL` cannot run cleanup.

Run this scenario natively on Linux amd64/arm64, macOS amd64/arm64, and Windows Terminal amd64/arm64
before release. Record architecture, terminal emulator/version, standard-console handle status, and any
protocol limitation in the platform evidence file.

## Acceptance

- All automated quality gates pass.
- On the Linux amd64 reference runner (2 logical CPUs, 4 GiB free, local temporary SQLite, 80x24, no
  concurrent load), after one warm-up at least 19/20 refresh, expand and collapse measurements remain
  below one second using the timing boundaries in SC-004.
- All eight normal controls and three secret prompt types pass the closed ASCII/Unicode, shortcut,
  cursor, selection, separator, limit and rejection corpora.
- No canary secret occurs in output, errors, logs or snapshots.
- Native terminal modes are restored after submit, cancel, context cancellation, SIGINT/SIGTERM and
  injected failures.
