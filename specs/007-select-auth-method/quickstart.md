# Quickstart Validation: Selección directa del método de autenticación

## Prerequisites

- Repository root: `/home/fallen/code/orza`
- Nix with flakes enabled, or Go 1.26.6 matching `go.mod`
- A VT-capable terminal at least 40x12 for manual validation
- Synthetic `.invalid` hosts and a disposable catalog only; do not use real credentials, identity files, or SSH sessions

Enter the pinned development environment:

```sh
nix develop
go version
```

The reported Go version should match the module's Go 1.26 toolchain family.

## Contract References

- [Data model](data-model.md): typed selection, dependent drafts, and request projection
- [TUI selector contract](contracts/tui-auth-method-selector.md): rendering, input, focus, and validation matrix
- [Research](research.md): implementation decisions and rejected alternatives

## Fast Validation

Run focused form, input, accessibility, and request tests:

```sh
go test ./internal/tui \
  -run 'Test(ConnectionForm|RoutesPaste|EmbeddedConnectionForm|Dirty|NoColor|Responsive|Resize|SC003|SC006|SC007)' \
  -count=1
go test ./internal/app -run 'Test(ConnectionService|Credential)' -count=1
go test ./tests/integration -run 'TestTUIConnection' -count=1
```

Expected outcomes:

- New forms show `[Agent] Key Password`; edit forms select their persisted method.
- Left/Right reaches every method and wraps at both ends without a separate activation step.
- Method names, arbitrary text, editing keys, and paste leave selection and all other form state unchanged.
- Identity and Remember appear only for their applicable method and return unchanged after temporary hiding.
- Create/update requests include only applicable data; a hidden checked Remember never requests or stores a password for Agent or Key.
- Validation and persistence failures retain selector state, focus, drafts, and actionable errors.

## Complete Automated Validation

Run the repository quality gates:

```sh
gofmt -w cmd internal tests
go test ./...
go test -race ./...
go vet ./...
go build ./...
staticcheck ./...
govulncheck ./...
```

Run the reproducible pull-request gate:

```sh
nix flake check --no-update-lock-file --keep-going -L
```

All commands must pass. Review formatting changes before retaining them. Selector tests must not require network access, a real SSH server, a private key, password input, or a native credential store.

## Responsive Acceptance

Run the build-tagged responsive suite after routine tests pass:

```sh
go test -tags acceptance ./internal/tui \
  -run '^TestSC007EveryScaleNodeAcrossTwelveSizesTwentyRuns$' \
  -count=1 -v
```

Expected outcomes:

- The selected method, focused row, dependent draft, and logical viewport state survive every supported resize.
- Agent, Key, and Password remain complete at 40x12, 79x24, and 80x24 in color and no-color modes.
- A focused Method row remains visible when the form overflows and the scrollbar remains informational.
- The 40x12→39x11→40x12 transition restores the exact selector and dependent state before accepting input.

## Manual Keyboard Validation

Create an isolated catalog and start the TUI:

```sh
export XDG_DATA_HOME="$(mktemp -d)"
go run ./cmd/orza folder create /selector-test
go run ./cmd/orza
```

Validate using only the keyboard:

1. Select `/selector-test`, press `n`, and Tab to Method.
2. Confirm `Agent`, `Key`, and `Password` are all visible, only Agent is bracketed, and `>` independently marks row focus.
3. Press Left once and confirm Password is selected; press Right once and confirm Agent returns.
4. Cycle to Key, enter a synthetic path such as `/tmp/non-secret-test-key`, cycle through Password to Agent and back to Key, and confirm the path is retained.
5. Cycle to Password, toggle Remember, cycle away and back, and confirm the check remains; do not save a remembered password during manual validation.
6. While Method is focused, type `agent`, `q`, and `?`, press Backspace/Delete, and paste `password`; confirm nothing changes and no browser command opens.
7. Press F1 and confirm Help documents cyclic Left/Right method selection; close Help and confirm form state is unchanged.
8. Repeat in an edit form whose saved method is Key or Password and confirm the persisted method is initially selected.
9. Resize through 40x12, 79x24, and 80x24, then through 39x11 and back to 40x12; confirm state and visibility recover exactly.
10. Repeat with `NO_COLOR=1`; confirm focus and selection remain distinguishable without ANSI color.
11. Cancel the form and confirm no connection or credential change was persisted.

Do not initiate a connection. The feature requires no network operation or real secret for validation.
