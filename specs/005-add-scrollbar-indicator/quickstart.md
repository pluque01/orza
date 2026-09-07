# Quickstart Validation: Indicador de barra de desplazamiento

## Prerequisites

- Repository root: `/home/fallen/code/orza`
- Nix with flakes enabled, or Go 1.26.5 matching `go.mod`
- A VT-capable terminal at least 40x12 for manual validation
- Synthetic catalog data only; no production hosts, credentials, or SSH sessions are required

Enter the pinned development environment:

```sh
nix develop
go version
```

The reported Go version should match the module's Go 1.26 toolchain family.

## Contract References

- [Data model](data-model.md): projection, geometry, and state invariants
- [TUI scrollbar contract](contracts/tui-scrollbar.md): visual placement, keyboard behavior, and test matrix
- [Research](research.md): decisions and rejected alternatives

## Fast Validation

Run focused viewport and cross-surface tests:

```sh
go test ./internal/tui -run 'Test(Viewport|Scrollbar|US5.*Overflow|SC007)' -count=1
```

Expected outcomes:

- Fit cases reserve no scrollbar column.
- Overflow cases show a one-column track adjacent to the right border.
- Thumb length/top exactly match half-up formulas for the 36-case SC-003 matrix, including one-row tracks and extreme inputs.
- No rendered frame contains `↑ more` or `↓ more`; an overflowing Actions panel shows exactly `Hidden actions — ? Help` and no scrollbar.
- Active tree rows, fields, errors, picker choices, and fixed modal controls remain visible.

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

When Nix is available, validate the pinned package, formatting, tests, analysis, vulnerability database, and native smoke test:

```sh
nix flake check
```

All commands must pass. Scrollbar tests must not require network access, a real SSH server, credentials, or a platform credential store.

### Current Toolchain Advisory

- On 2026-09-04, the pinned `nixpkgs-26.05-darwin` development shell provides Go 1.26.5. The current vulnerability database reports reachable `GO-2026-5972` in its standard-library `encoding/asn1`, fixed in Go 1.26.6. The separately reported `golang.org/x/crypto/ssh` advisories `GO-2026-6354` and `GO-2026-6355` are addressed by the pinned module version.
- `govulncheck ./...` therefore remains blocked until the pinned Nix toolchain provides Go 1.26.6 or newer. This external toolchain limitation does not waive the pre-merge vulnerability gate.

## Scale Acceptance

Run the build-tagged viewport acceptance suite after routine tests pass:

```sh
go test -tags acceptance ./internal/tui -run '^TestSC007EveryScaleNodeAcrossTwelveSizesTwentyRuns$' -count=1 -v
```

Expected outcomes:

- The existing 1,000-connection, 100-folder, depth-10 catalog remains bounded at all twelve responsive sizes.
- Every overflowing viewport reports and renders its own proportional bar.
- Tree selection and other active content remain visible.
- Unicode lines truncate within the reduced text width using `…`.
- Marker rows no longer reduce visible content capacity.

## Manual Keyboard Validation

Use an isolated temporary catalog location and start the TUI:

```sh
export XDG_DATA_HOME="$(mktemp -d)"
go run ./cmd/orza folder create /scroll-test
for i in $(seq 1 30); do
  go run ./cmd/orza connection create "/scroll-test/host-$i" \
    --host "host-$i.example.invalid" --user tester --auth agent
done
go run ./cmd/orza
```

Validate using only the keyboard:

1. Expand `/scroll-test` and move through Tree with arrows or `j`/`k`.
2. Confirm the thumb starts at the top, moves downward with the visible range, and touches the bottom at the final row.
3. Use `Tab` to focus Details and scroll with `j`/`k`, `g`, and `G`; confirm only the Details bar changes.
4. Open Help and a confirmation with enough body text to overflow; confirm fixed close/cancel controls have no track beside them and remain visible.
5. Repeat 20 times the width sequence `40→60→79→80→100→160→80→79→40` at heights 12 and 24; confirm bars appear/disappear without losing focus, selection, logical offset, or active visibility.
6. Repeat 20 times `40x12→39x11→40x12`; confirm the undersized view has no bar, then restore size and confirm state and applicable bars return before the next input.
7. Repeat with `NO_COLOR=1`; confirm `│` and `█` remain visibly different.
8. Confirm no scrollbar receives focus and no mouse action is needed or advertised.

Do not confirm connection attempts. The `.invalid` targets are synthetic and no network operation is part of this validation.

## Visual Acceptance Record

Before merge, a maintainer runs SC-006 with 10 participants who did not contribute to the feature, terminal 80x24, and the same synthetic 30-line Help body in color and no-color modes. Randomly present start, middle, and end and ask exactly: “¿Hay contenido oculto y estás al inicio, en medio o al final?”. Record only:

- Whether hidden content was identified within 5 seconds.
- Whether the participant correctly classified the position as start, middle, or end within 5 seconds.

At least 9 of 10 participants must identify both overflow and position in less than 5 seconds in all six cases. Do not record names, terminal content, host metadata, or other personal data. Store the aggregate evidence in `validation/sc006.md`.
