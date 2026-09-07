# Quickstart: SSH Startup Diagnostics

## Prerequisites

- Go 1.26.5 through `nix develop`.
- Existing feature-002 tests passing.
- No production credentials; use only controlled canaries and the in-process SSH server.

## Automated validation

```sh
nix develop -c go test ./internal/app ./internal/sshclient ./internal/tui ./internal/cli
nix develop -c go test ./tests/integration -run 'SSH|TUI|CLI|Terminal'
nix develop -c go test -race ./...
nix develop -c go vet ./...
nix develop -c staticcheck ./...
nix develop -c govulncheck ./...
nix flake check
```

Run sanitizer fuzzing for a bounded review interval:

```sh
nix develop -c go test ./internal/app -run '^$' -fuzz '^FuzzSanitizeSSHFailureDetail$' -fuzztime 30s
```

All formatting, complete-suite, race, vet, staticcheck, govulncheck, and Nix gates must pass before merge.
An unavailable environment does not waive a gate; leave it pending and run it in CI or another capable
environment. Native-platform evidence is recorded separately and does not replace mandatory local gates.

## Category matrix

Exercise deterministic injected causes for all ten categories and establish category, stage, non-empty
summary/recommendation, optional safe detail, captured target, precedence through wrappers/joins, and
deadline-versus-cancel behavior. Use typed network errors rather than public network timing.

Measure diagnostic latency with a monotonic or injected clock from the return of `Connect` after cleanup and
terminal restore to complete model update/stderr write. Every controlled TUI, human CLI, and structured CLI
case must remain below one second without another network operation.

## Automated boundaries

The automated suite is the primary verification mechanism, not a substitute for steps that could have been
tests. It currently covers:

- All ten category projections across TUI, human stderr, and structured stderr, including stable stages,
  captured target, broad code compatibility, one-object JSON output, stdout separation, sub-second
  presentation, hidden-by-default detail, and no second network operation.
- TUI `d`, `r`, `e`, `Esc`, and `q` behavior; captured-ID recovery, changed/missing targets, fresh confirmation,
  and prevention of network or persistence before explicit acceptance.
- Full, reduced, tiny, and no-color render priorities, including 80x24 and recovery-control visibility.
- A real SSH protocol boundary over loopback using an in-process server: rejected password authentication is
  `authentication_denied` at `authentication`; an active session's remote nonzero status remains a session
  result without startup fields; terminal-state cleanup is asserted through the instrumented terminal.
- A real Unix PTY boundary, when `script(1)` is installed: no-echo secret input, bracketed-paste cleanup,
  terminal restoration, and SIGINT/SIGTERM exits 130/143. If `script(1)` is unavailable, this mandatory gate
  remains pending and must run in a capable CI or review environment; a skipped test is not approval.
- Typed Unix and Windows resolver/socket classifications without localized string matching, cross-surface
  raw-cause canaries, and fuzz checks for malformed UTF-8, ANSI/control/bidirectional input, sensitive terms,
  and oversized detail. The implementation has no diagnostic persistence or logging path.

## Confidentiality

Inject canaries into password, passphrase, key content, callback errors, wrapped causes, server messages,
ANSI/control/bidi payloads and >256-Unicode-code-point text. Search TUI views, human/JSON outputs, returned safe errors,
diagnostics, logs and test snapshots; no canary or raw cause may appear.

## Security review

### Threat assumptions

- Catalog path and endpoint are user-visible metadata, but credentials, passphrases, private-key material,
  credential references, arbitrary server/library/operating-system text, resolved addresses, and remote
  session output are untrusted for diagnostic presentation.
- A failure can contain wrapped or joined attacker-influenced causes, malformed UTF-8, terminal escapes,
  controls, bidirectional controls, long input, or localized platform text. Presentation receives only the
  controlled projection and fixed detail allowlist; sanitization does not make arbitrary causes displayable.
- The local account, terminal emulator, operating system, SSH server, and network can fail or behave
  maliciously. This feature does not defend a compromised local account or host, and does not alter host-key,
  authentication, credential-store, catalog-permission, or terminal trust policy.
- Diagnostic state is attempt-local. The feature adds no persistence, telemetry, history, or application
  logging, and recovery actions do not write diagnostic data.

### Manual verification boundaries

Manual security verification is required only where a portable automated test cannot reproduce the native
boundary:

- On Windows, Linux, and macOS, inspect the release artifact in representative native terminal emulators to
  confirm human-visible rendering, keyboard delivery, focus, resize behavior, and terminal restoration at
  80x24, reduced dimensions, and with color disabled. Snapshots and pseudo-terminals cannot reproduce every
  emulator, console host, font, or input-event translation.

Native resolver/socket classification, real SSH behavior, process statuses, and terminal APIs are not manual
exceptions: run their automated tests on Windows, Linux, and macOS for amd64 and arm64. Cross-builds and
synthetic errno tests do not replace those native automated gates; unavailable hardware leaves them pending.

Record the OS, architecture, terminal, artifact, and observed result. Manual evidence supplements automated
gates and cannot waive, replace, or mark any mandatory gate as passed; unavailable hardware leaves that gate
pending.

## Acceptance

- Ten-category matrix passes in TUI, human CLI and structured CLI.
- Every controlled failure is visible/emitted within one second after completion.
- Retry/edit/back/detail are keyboard-only and target-stable.
- Sanitizer fuzz invariants hold without panic or disclosure.
- Complete tests, static analysis, vulnerability scan, Nix checks and cross-builds pass.
- Required native manual boundaries are recorded; no mandatory automated or manual gate is waived.

## Validation Results (2026-08-06)

- Formatting check and `go test ./...`: PASS.
- `go test -race ./...`, `go vet ./...`, `staticcheck ./...`, and `govulncheck ./...`: PASS; no called vulnerabilities found.
- `nix flake check`: PASS for all checks compatible with the x86_64-linux runner; Nix reported Darwin and aarch64-linux checks as incompatible with this host, not failed.
- `FuzzSanitizeSSHFailureDetail` for 30 seconds: PASS after adding the deterministic 256/257-code-point boundary regression.
- `CGO_ENABLED=0 go build ./...`: PASS for Linux, Windows, and macOS on amd64 and arm64.
- Native Windows/macOS terminal-emulator evidence remains a release-environment gate as described above; cross-build success does not mark it complete.
