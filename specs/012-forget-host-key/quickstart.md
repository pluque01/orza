# Quickstart: Validate Forget Host Key

## Prerequisites

1. Build or run Orza from this repository.
2. Use a test catalog location and create a connection to a test SSH host.
3. Connect once and choose persistent trust so the endpoint has app-owned host trust.
4. Ensure the test endpoint is not solely trusted through standard `known_hosts`, or account for that independent trust source in the expected connection result.

## Automated Validation

Run the full suite:

```bash
go test ./...
```

The focused tests must cover catalog conditional deletion, the application scope and conflict rules, CLI human and JSON results, and TUI action and confirmation behavior.

## CLI Scenario

1. Inspect the connection and record its path or ID.
2. Run `orza connection forget-host-key PATH_OR_ID`.
3. Verify a successful human result identifies the endpoint as forgotten, without a prompt.
4. Run the command again and verify it succeeds with a no-app-owned-key result.
5. Run with `--json` and verify the standard success envelope contains the endpoint and `forgotten` state as defined in [the command contract](contracts/forget-host-key.md).
6. When testing concurrency, replace or remove trust after reading its revision, then run with the stale `--if-revision`; verify conflict and no unintended deletion.

## TUI Scenario

1. Select a connection whose endpoint has app-owned host trust.
2. Open **Forget host key** from the connection actions.
3. Verify the confirmation names `host:port`, states that only app-owned trust is removed, and warns that a later connection may request trust again.
4. Press Enter or Escape and verify the TUI returns to the connection without changing trust.
5. Reopen the action, press `y`, and verify the success state and that the next connection follows normal host-trust evaluation.
6. Repeat with no app-owned trust and verify a stable no-change result.

## Trust Boundary Check

Before and after every scenario, inspect the user's standard SSH trust files separately if they are present. Their contents must be identical; only Orza's app-owned trust is in scope for this feature.
