# Linux Native Validation

**Status**: Partial; amd64 automated coverage complete, arm64 native and interactive scenarios pending.

## Linux amd64

- Full Go tests and race detector: PASS.
- Unix PTY tests for remembered password, SSH password, and private-key passphrase: PASS.
- PTY SIGINT/SIGTERM cleanup and terminal restoration: PASS.
- Tree performance acceptance: 20/20 for refresh, expand, and collapse.
- `NO_COLOR` and 80x24 behavior have automated model coverage.

## Pending

- Run the 20/20 interactive keyboard-only sequence in a real amd64 terminal and record emulator/version.
- Run all native scenarios on a Linux arm64 host. The arm64 Nix cross-build passes but is not native evidence.
