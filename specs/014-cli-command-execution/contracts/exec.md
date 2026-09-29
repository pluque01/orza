# CLI Contract: Execute Remote Command

## Command

```text
orza exec PATH_OR_ID [--timeout DURATION] -- COMMAND
```

`exec` runs exactly one remote shell-text command through one saved connection. It is non-interactive: it does not require a terminal, start the TUI, request a PTY, prompt for trust or credentials, or change the saved connection.

## Inputs

| Input | Required | Rules |
|---|---|---|
| `PATH_OR_ID` | Yes | Existing absolute catalog path or canonical connection ID. It must identify exactly one connection. |
| `--timeout DURATION` | No | Positive Go duration. Defaults to `5m`; the deadline covers the command operation and ends it safely when reached. |
| `--` | Yes | Separates the selector and flags from the single remote command argument. |
| `COMMAND` | Yes | Non-empty remote shell text. The caller quotes it for the local shell; Orza passes the resulting text unchanged to the selected remote endpoint. Pipes and redirections are interpreted remotely, never locally by Orza. |
| standard input | No | Forwarded directly to the remote command. When the caller provides no input, the remote command receives EOF. |

Global `--json` is invalid with `exec`. The command streams remote output and cannot also write one JSON response. `--no-color` remains accepted but does not alter remote output.

## Stream Behavior

| Channel | Behavior |
|---|---|
| Standard input | Forwarded from the invoking process without a local prompt. |
| Standard output | Forwarded to the invoking process's standard output as received. |
| Diagnostic output | Forwarded to the invoking process's standard error as received. |
| Local diagnostics | Written to standard error using existing safe error presentation. They never include credentials, private key material, passphrases, or session secrets. |

No command text or I/O channel is persisted or accumulated as a command result.

## Exit Results

| Condition | Process exit result |
|---|---|
| Remote command succeeds | `0` |
| Remote command exits with a valid nonzero status | The exact remote status from `1` through `255` |
| Invalid arguments, empty command, invalid selector, invalid timeout, or `--json` | Existing usage status `2` |
| Saved connection absent | Existing not-found status `3` |
| Saved connection changed after resolution | Existing conflict status `4` |
| Invocation cancellation | Existing canceled status `5` |
| Unknown, changed, revoked, or rejected host identity; unavailable or rejected authentication | Existing security status `6` |
| Catalog failure | Existing catalog status `7` |
| Deadline expiry, network, SSH negotiation, session, or stream failure | Existing transport status `10` |
| Process signal handled by the root command | Existing signal result `130` for interrupt or `143` for termination |

Local statuses may equal a status returned by a remote command. Orza never remaps a completed remote command's original status to reserve a local-only value.

## Non-Interactive Safety

- Unknown or changed host identity fails before authentication and command execution; automation cannot approve or persist trust through `exec`.
- Only agent authentication, unencrypted keys, and remembered credentials available through the existing secure store may satisfy authentication without prompting.
- If a password or passphrase would need user entry, `exec` fails with the security status rather than reading a secret from the terminal or command stream.
- On timeout, cancellation, or interruption, the command session and all connection resources are released before the command returns.
