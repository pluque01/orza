# Data Model: CLI Command Execution

This feature adds transient application values only. It does not add catalog tables, alter saved connections, or persist command text, I/O, outcomes, or credentials.

## Existing Entity: Saved Connection

| Field | Meaning | Rules for this feature |
|---|---|---|
| `id` | Stable opaque connection identity | Captured after selector resolution and used for execution. |
| `revision` | Connection version | Captured before network activity; a changed connection rejects the execution attempt. |
| `path` | Displayable catalog path | Used in safe diagnostics; never used as a shell value. |
| `host` and `port` | SSH endpoint | Used for host verification and network connection. |
| `username` and authentication method | SSH access configuration | Reused without modifying the saved connection. |

## New Value: Remote Command Request

| Field | Meaning | Validation |
|---|---|---|
| connection selector | User-supplied path or ID | Must resolve to exactly one saved connection before execution. |
| captured connection ID | Immutable execution target | Must match the resolved connection. |
| expected revision | Version observed at resolution | Must match before SSH activity; stale state fails without network execution. |
| command text | One remote shell command | Required, non-empty, passed unchanged to the remote SSH exec request; not interpreted locally. |
| deadline | Maximum operation duration | Defaults to 5 minutes; caller override must be a positive duration. |
| standard input | Caller-supplied byte stream | Forwarded directly; EOF is forwarded as closed remote input. |
| standard output and diagnostic output | Caller-provided output streams | Remain separate and are streamed without retention. |

## New Value: Command Execution Result

| Field | Meaning | Rules |
|---|---|---|
| state | Final lifecycle state | `succeeded`, `failed`, or `canceled`. |
| outcome | Completion classification | Success, remote-command failure, transport failure, security failure, or cancellation/timeout. |
| started at | Timestamp after the remote command starts | Used for diagnostics and tests; not persisted. |
| remote exit status | Remote process result | Present only when the remote command ran and ended with a nonzero valid status. Preserve values 1 through 255 exactly. |
| failure presentation | Safe local failure context | Excludes command text secrets, passwords, passphrases, private keys, and session secrets. |

## Relationships and Lifecycle

1. The CLI parses the selector, one command text argument, and optional timeout.
2. The application service resolves the saved connection, captures its ID and revision, and rejects a stale revision before any network work.
3. SSH host verification runs before authentication. Unknown, changed, and revoked identities stop the lifecycle without a command session.
4. Authentication uses only non-interactive sources. Credential unavailability stops the lifecycle without terminal prompting.
5. The SSH command runner creates one non-PTY session, attaches the three caller streams, and executes the command text.
6. Normal completion returns success or the exact remote status. Cancellation or deadline closes the session, waits for completion, releases all resources, and returns a local classified failure.
7. No lifecycle state is persisted and the saved connection is unchanged in every outcome.

## Concurrency and Ownership Rules

- Each invocation owns exactly one transient command session and its associated network and authentication resources.
- The catalog revision pin prevents an execution based on stale saved-connection data.
- Output streams are consumed as they arrive and are never retained in an unbounded result buffer.
- Host trust and credential records remain owned by their existing services; execution neither modifies nor creates them.
