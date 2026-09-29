# Quickstart: Validate CLI Command Execution

## Prerequisites

1. Build from this repository with the declared Go toolchain.
2. Use a disposable catalog and a reachable test SSH server.
3. Create one saved connection using agent authentication, an unencrypted test key, or a password already available through the platform credential store.
4. Approve the test server's host identity through the existing interactive connection workflow before using `exec`. Do not place passwords or passphrases in scripts, command lines, or environment variables.

## Automated Validation

Run the complete suite:

```bash
go test ./...
```

Focused coverage must prove exact command forwarding, standard-input forwarding, separate streamed output channels, default and custom deadlines, remote-status preservation, local failure classification, non-PTY operation, host verification before secret access, no terminal prompt, stale-revision rejection, and deterministic cleanup.

## CLI Scenarios

1. Run an immediate command on a trusted saved connection:

   ```bash
   go run ./cmd/orza exec /test/server -- 'printf "ready\n"'
   ```

   Verify `ready` appears on standard output, no interactive interface starts, and the process exits `0`.

2. Verify remote shell syntax and separated channels:

   ```bash
   go run ./cmd/orza exec /test/server -- 'printf "out\n"; printf "err\n" >&2; printf "one\ntwo\n" | tail -n 1'
   ```

   Verify standard output contains `out` and `two`, diagnostic output contains `err`, and output is visible while the command is running.

3. Verify standard-input forwarding:

   ```bash
   printf 'payload\n' | go run ./cmd/orza exec /test/server -- 'cat'
   ```

   Verify the remote command receives and returns `payload` without a prompt.

4. Verify remote exit-status preservation:

   ```bash
   go run ./cmd/orza exec /test/server -- 'exit 23'
   status=$?
   ```

   Verify `status` is `23`.

5. Verify timeout cleanup with a short override:

   ```bash
   go run ./cmd/orza exec /test/server --timeout 100ms -- 'sleep 10'
   ```

   Verify the process exits with the documented transport status, reports a timeout safely on standard error, and does not leave an active session.

6. Verify non-interactive host-trust failure by using a test connection to an unknown or changed host. Verify no command runs, no credential prompt occurs, no trust is saved, and the process returns the security status.

7. Run `go run ./cmd/orza --json exec /test/server -- 'true'` and verify it fails as invalid usage without emitting a mixed JSON and command-output response.

## Regression Checks

1. Run the existing interactive `orza connect PATH_OR_ID` flow from a terminal and verify PTY, raw mode, host-trust decisions, and terminal restoration remain unchanged.
2. Run a command with large standard and diagnostic output using the in-process SSH integration server. Verify both streams remain distinct and no result buffer retains the full payload.
3. Cancel an active command with the existing root signal behavior. Verify resources are released and the root signal exit result is preserved.
