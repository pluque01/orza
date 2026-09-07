# Quickstart Results: Gestión de conexiones SSH

**Date**: 2026-07-29
**Host**: Linux x86_64
**Catalog**: Disposable XDG data directory under `/tmp/opencode`

## Reproducible Environment and Gates

| Validation | Result |
|------------|--------|
| Go toolchain from `nix develop` | PASS, Go 1.26.5 |
| `go test ./...` | PASS |
| `go test -race ./...` | PASS |
| `go vet ./...` | PASS |
| `staticcheck ./...` | PASS |
| `govulncheck ./...` | PASS, zero called vulnerabilities |
| `nix flake check --print-build-logs` | PASS, eight checks |

The race gate was repeated after making the resize test wait for the resize event instead of racing an
immediate remote exit.

## Release Builds

| Artifact | Build | Native execution |
|----------|-------|------------------|
| Linux amd64 | PASS | PASS |
| Linux arm64 | PASS | Not available on this host |
| Windows amd64 | PASS | Not available on this host |
| Windows arm64 | PASS | Not available on this host |
| macOS amd64 | Evaluation PASS | Requires an x86_64 Darwin builder |
| macOS arm64 | Evaluation PASS | Requires an aarch64 Darwin builder |

## CLI CRUD and Conflicts

The release binary was executed against an isolated catalog. The following sequence passed:

1. Printed version `0.1.0`.
2. Created `/quickstart-validation` and `/quickstart-validation/team`.
3. Created an agent-auth connection at `/quickstart-validation/team/demo`.
4. Read, renamed and moved the connection while retaining its stable ID.
5. Listed folders before connections and emitted a redacted JSON projection.
6. Rejected `--if-revision 1` after the connection reached revision 3 with exit status 4.
7. Recursively deleted two folders and one connection without affecting the root.

No command modified `~/.ssh/config`; the integration security test snapshots and verifies this file.

## SSH and Terminal Boundary

`TestSSHClientAgainstInProcessServerRestoresTerminalAndReturnsRemoteStatus` passed against a controlled
in-process SSH server. It verifies host trust before authentication, PTY/session ownership, direct
streaming, remote status mapping and terminal restoration. Unit tests additionally cover changed and
revoked keys, agent/key/password authentication, cancellation, resize and every cleanup stage.

## TUI

Automated Bubble Tea model and integration scenarios passed for:

- Keyboard-only root connection CRUD and help.
- Empty, 80x24, narrow-terminal and `NO_COLOR` golden views.
- Validation, discard and destructive confirmations.
- Stale-write rejection and explicit reload.
- Host trust before secret prompts.
- Recoverable pre-session failure and active-session handoff.
- Nested breadcrumbs, move picker exclusions and recursive folder confirmation.

An interactive human usability session was not available in this noninteractive agent environment.

## Native Linux Boundaries

| Scenario | Result |
|----------|--------|
| Headless Secret Service fails closed | PASS |
| Secret Service real lifecycle | SKIP: `org.freedesktop.secrets` not activatable |
| Missing, absent and controlled SSH-agent socket | PASS |
| Catalog directory/file modes | PASS |
| Linux release executable smoke | PASS |

The missing desktop Secret Service never falls back to a file or catalog secret. Passwords remain
session-only in this environment.

## Outstanding Native Evidence

- Windows Credential Manager, named pipe, DACL and console tests must run on Windows runners.
- macOS Keychain, terminal and release tests must run on Intel and Apple Silicon Darwin runners.
- Linux Secret Service lifecycle must run in a desktop session with an activatable service.
