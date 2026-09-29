# Implementation Plan: CLI Command Execution

**Branch**: `014-cli-command-execution` | **Date**: 2026-09-29 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/014-cli-command-execution/spec.md`

## Summary

Add a non-interactive `orza exec` command that runs one remote shell command through a saved connection. A dedicated application service and non-PTY SSH runner will reuse catalog resolution, revision pinning, host verification, credential recovery, SSH authentication, and safe failure presentation. The command streams standard input, output, and diagnostics directly, applies a configurable five-minute default timeout, preserves remote exit statuses, and never prompts, launches the TUI, or weakens host trust.

## Technical Context

**Language/Version**: Go 1.26.0 with toolchain Go 1.26.6

**Primary Dependencies**: Cobra 1.10.2 for CLI parsing; `golang.org/x/crypto/ssh` for SSH transport; existing catalog, credential, host-key, terminal, and application packages

**Storage**: Existing local SQLite catalog and native credential store are read only for this feature; no command, output, or credential data is added or persisted

**Testing**: Go standard test tooling (`go test ./...`), package unit tests, in-process SSH integration tests, and existing stream/security regression suites

**Target Platform**: Linux, macOS, and Windows on amd64 and arm64; a terminal is not required for `exec`

**Project Type**: Local Go CLI and terminal application

**Performance Goals**: For a reachable saved connection and immediate remote command, 95% of 100 consecutive invocations complete within 5 seconds excluding remote command duration; output is observable before command completion

**Constraints**: One saved connection and one remote shell-text command per invocation; five-minute default configurable timeout; direct bounded streaming of stdin/stdout/stderr; no local shell interpretation, PTY, terminal raw mode, resize handling, interactive prompt, or `--json` output mode

**Scale/Scope**: Reuses catalog behavior proven for 1,000 connections, 100 folders, and 10 nesting levels; each invocation owns one transient SSH command session and retains no command output

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Pre-design gate: PASS**

| Constitutional requirement | Design response |
|---|---|
| Secure SSH operations | Reuse the existing host-verification gate before authentication. Non-interactive execution supplies no trust decision, so unknown or changed identities fail without running the command or persisting trust. Secrets remain in the credential boundary and are never rendered. |
| Terminal-first usability | `exec` is an explicit documented non-interactive behavior. It does not change interactive `connect` or TUI behavior, and it does not capture or alter terminal state. |
| Predictable state and failure handling | A dedicated command runner owns one session. Context deadline or cancellation closes the session and waits for completion before closing transport, raw connection, and authentication resources. |
| Behavior-focused testing | Add CLI, application, SSH-client, and in-process SSH integration coverage for success, output channels, stdin, timeout, cancellation, trust rejection, unavailable secrets, exact remote status, and cleanup. |
| Simplicity and maintainability | Add one command-specific service and runner port rather than weakening the deliberately interactive `ConnectService`. Reuse existing adapters and do not add dependencies or persistent state. |

**Post-design gate: PASS**

The Phase 1 request/result model is transient, the CLI contract rejects incompatible JSON output, and the design has no constitutional exception or complexity violation.

## Project Structure

### Documentation (this feature)

```text
specs/014-cli-command-execution/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── quickstart.md        # Phase 1 output (/speckit.plan command)
├── contracts/
│   └── exec.md           # CLI command contract
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
```text
cmd/orza/
├── main.go                         # Dependency construction and process signal handling
└── main_test.go                    # Root-process and signal behavior

internal/
├── app/
│   ├── command.go                  # New non-interactive command service
│   ├── ports.go                    # SSH command runner port
│   ├── types.go                    # Transient command request/result values
│   └── *_test.go                   # Application behavior tests
├── cli/
│   ├── root.go                     # Register exec and inject service
│   ├── exec.go                     # Command schema, timeout, stream wiring, error mapping
│   └── exec_test.go                # CLI contract tests
├── sshclient/
│   ├── client.go                   # Shared connection setup and command execution entry point
│   ├── session.go                  # Non-PTY command session lifecycle
│   └── *_test.go                   # I/O, cancellation, exit-status, cleanup tests
└── hostkey/, credential/, catalog/ # Existing adapters reused without schema changes

tests/integration/
├── ssh_server_test.go               # In-process SSH server exec support
└── security_test.go                 # Host-trust, secrets, stream, and timeout coverage
```

**Structure Decision**: Retain the existing single Go application. Add command-specific behavior beside the existing interactive connection flow while sharing the existing application and transport boundaries. No catalog migration, new module, or third-party dependency is required.

## Complexity Tracking

No constitutional violations or complexity exceptions require tracking.
