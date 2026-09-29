# Research: CLI Command Execution

## Decision: Add a dedicated `exec` command and command service

Expose non-interactive execution as `orza exec PATH_OR_ID [--timeout DURATION] -- COMMAND`. The CLI delegates to a command-specific application service and SSH command-runner port rather than changing the interactive `connect` command or `ConnectService`.

**Rationale**: `connect` intentionally requires an interactive terminal, trust-decision prompt, PTY, raw terminal mode, and resize forwarding. A separate path makes the no-terminal guarantee explicit while retaining the existing interactive workflow unchanged.

**Alternatives considered**:

- Add a non-interactive flag to `connect`: rejected because it would combine incompatible terminal and trust behaviors in one command and service.
- Invoke the system `ssh` executable: rejected because it bypasses the existing catalog, trust, credential, error, and test boundaries.

## Decision: Pass one remote shell-text argument unchanged

Require one command argument after `--`. The local CLI passes this exact text to the SSH exec request and does not invoke or interpolate a local shell. The selected remote endpoint interprets pipes, redirections, and other shell syntax.

**Rationale**: A single already-quoted argument preserves caller intent and supports the specified remote shell behavior while avoiding local shell interpretation of untrusted command, host, user, and path values.

**Alternatives considered**:

- Join arbitrary trailing arguments: rejected because it changes whitespace and quoting semantics.
- Accept only executable-plus-argument values: rejected because the specification requires remote shell syntax.

## Decision: Reuse host verification but provide no non-interactive trust decision

The command service resolves and revision-pins the selected connection, then uses the existing host-verification gate. It supplies no decision callback, so unknown, changed, or revoked identities fail before authentication and command execution.

**Rationale**: The constitution requires explicit host-trust decisions. A non-interactive invocation cannot make one safely, and it must never silently trust or persist a presented key.

**Alternatives considered**:

- Trust unknown hosts for one command: rejected because it weakens the SSH trust boundary.
- Prompt through standard input: rejected because the feature must complete without terminal input and standard input is reserved for command data.

## Decision: Use only non-interactive credential sources

The command service permits SSH agent authentication, unencrypted keys, and remembered passwords supplied by the secure credential store. If authentication needs a password or key passphrase not available from those sources, it returns a classified credential failure and never calls terminal secret prompting.

**Rationale**: This permits established unattended credentials without exposing or requesting secrets in scripts, agent output, or the terminal.

**Alternatives considered**:

- Read a password from command arguments or environment variables: rejected because it exposes secret material.
- Prompt when standard input is a terminal: rejected because it would make behavior depend on terminal presence and violate the non-interactive contract.

## Decision: Stream I/O directly and reject JSON mode

Wire the invoking process's stdin, stdout, and stderr directly to the SSH command session. Output is not accumulated. Reject global `--json` for `exec` because streamed remote stdout cannot coexist with a single CLI JSON envelope.

**Rationale**: Direct streams preserve stdin pipelines and output-channel separation, satisfy bounded-memory requirements, and give scripts the ordinary process interface they expect.

**Alternatives considered**:

- Buffer output and emit JSON on completion: rejected because it delays progress, risks memory growth, and violates streaming requirements.
- Mix JSON diagnostics with remote stdout: rejected because it corrupts command output and cannot be parsed reliably.

## Decision: Apply a command context deadline and preserve remote status

Create a five-minute context deadline by default, overridable with a positive duration per invocation. On deadline or cancellation, close the SSH command session, wait for it to end, and then release session, transport, raw connection, and authentication resources. A completed remote command preserves its status from 1 through 255 exactly; local failures use existing documented nonzero CLI statuses and may numerically collide with a remote status.

**Rationale**: Existing signal cancellation and resource-close ordering already establish safe lifecycle patterns. Preserving remote statuses matches SSH command behavior and avoids remapping valid remote results.

**Alternatives considered**:

- No default deadline: rejected because an unavailable endpoint or stalled command can block automation indefinitely.
- Reserve a local-only exit status by remapping remote statuses: rejected because arbitrary remote commands can return every valid status.

## Decision: Extend the in-process SSH test server for exec requests

Extend the existing integration server to handle SSH `exec` channel requests in addition to its current interactive shell requests. Use existing fake transport/session seams for deterministic timeout, output, cleanup, and exact-command tests.

**Rationale**: This tests the real protocol path and host-before-authentication ordering without an external server, while unit seams make failure timing deterministic.

**Alternatives considered**:

- Rely solely on mocked SSH sessions: rejected because it would not prove protocol-level exec behavior.
- Require a real external SSH host in tests: rejected because it is nondeterministic and unsuitable for the automated suite.
