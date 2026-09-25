# CLI Contract: Forget Host Key

## Command

```text
orza connection forget-host-key PATH_OR_ID [--if-revision REVISION] [--json]
```

The command removes only the app-owned SSH host trust associated with the target connection's host and port. It is non-interactive: it does not prompt and does not require a confirmation flag.

## Inputs

| Input | Required | Rules |
|---|---|---|
| `PATH_OR_ID` | Yes | Must identify one existing connection using the established selector format. |
| `--if-revision REVISION` | No | Positive trusted-host revision expected by an automation caller. A mismatch produces a conflict and no change. |
| `--json` | No | Uses the existing machine-readable output envelope. |

## Successful Results

Human-readable success when trust is removed:

```text
forgot app-owned host key for HOST:PORT
```

Human-readable idempotent no-op when no app-owned trust exists:

```text
no app-owned host key for HOST:PORT
```

JSON success when trust is removed:

```json
{"ok":true,"data":{"host":"HOST","port":22,"forgotten":true},"catalogRevision":42}
```

JSON success when no app-owned trust exists:

```json
{"ok":true,"data":{"host":"HOST","port":22,"forgotten":false}}
```

## Failure Results

| Condition | Result | State change |
|---|---|---|
| Invalid selector or non-positive revision | Existing invalid-usage response | None |
| Connection does not exist | Existing not-found response | None |
| Expected trust revision is stale, or the trust record changes after capture | Existing conflict response | None |
| Catalog operation fails | Existing catalog error response | None unless deletion was verified |

The command never edits `~/.ssh/known_hosts` or `~/.ssh/known_hosts2`.
