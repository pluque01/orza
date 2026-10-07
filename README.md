# Orza

[![CI](https://github.com/pluque01/orza/actions/workflows/ci.yml/badge.svg)](https://github.com/pluque01/orza/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/pluque01/orza)](https://github.com/pluque01/orza/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A terminal-native SSH client with a TUI

Navigate remote systems from your terminal.

Orza is a local, terminal-first SSH connection manager. It keeps connections in its own
hierarchical catalog, exposes the same catalog through a CLI and a keyboard-driven TUI, and opens
interactive SSH sessions and session-only TCP tunnels with host-key verification.

**Status:** pre-1.0 development. Interfaces and catalog migrations are tested, but releases are not
yet covered by a long-term compatibility promise. See [releases](https://github.com/pluque01/orza/releases),
[contribution guidance](CONTRIBUTING.md), [private security reporting](SECURITY.md), and the
[dependency vulnerability policy](SECURITY.md#dependency-findings).

The catalog is independent of OpenSSH configuration. **Orza never imports, rewrites, or appends
to `~/.ssh/config`.** It reads the user's standard `known_hosts` files for trust decisions, but stores
trust added through Orza in its own catalog.

## Requirements And Downloads

Orza supports an English, keyboard-only interface on Linux, macOS, and Windows, on `amd64` and
`arm64`. It requires a VT-capable terminal with normal screen, resize, keyboard, and bracketed-paste
behavior; the complete layout targets 80x24 and remains usable at 40x12. SSH compatibility is the
subset implemented by `golang.org/x/crypto/ssh`, not OpenSSH `ssh_config`; see
[Unsupported in v1](#unsupported-in-v1). Use `orza --no-color` or `NO_COLOR=1 orza` when ANSI color
is unavailable.

Select the archive matching the operating system and architecture reported by `uname -m`,
`go env GOARCH`, or Windows `$env:PROCESSOR_ARCHITECTURE`: `x86_64`/`AMD64` means `amd64`, and
`aarch64`/`ARM64` means `arm64`. For release `v1.2.3`, the exact assets are:

| Target | Release asset | Executable |
|---|---|---|
| Linux amd64 | `orza_v1.2.3_linux_amd64.tar.gz` | `orza` |
| Linux arm64 | `orza_v1.2.3_linux_arm64.tar.gz` | `orza` |
| macOS amd64 | `orza_v1.2.3_darwin_amd64.tar.gz` | `orza` |
| macOS arm64 | `orza_v1.2.3_darwin_arm64.tar.gz` | `orza` |
| Windows amd64 | `orza_v1.2.3_windows_amd64.zip` | `orza.exe` |
| Windows arm64 | `orza_v1.2.3_windows_arm64.zip` | `orza.exe` |

Every release also contains `orza_v1.2.3_checksums.txt`. Substitute an actual release tag below;
the [release page](https://github.com/pluque01/orza/releases) lists available versions.

### Linux

```sh
VERSION=v0.1.0
ARCH=amd64 # use arm64 on an arm64 system
ASSET="orza_${VERSION}_linux_${ARCH}.tar.gz"
CHECKSUMS="orza_${VERSION}_checksums.txt"
BASE="https://github.com/pluque01/orza/releases/download/${VERSION}"
curl -fLO "$BASE/$ASSET"
curl -fLO "$BASE/$CHECKSUMS"
grep "  $ASSET$" "$CHECKSUMS" | sha256sum --check --strict -
tar -xzf "$ASSET"
install -d "$HOME/.local/bin"
install -m 0755 orza "$HOME/.local/bin/orza"
"$HOME/.local/bin/orza" --version
"$HOME/.local/bin/orza" --help
```

Add `$HOME/.local/bin` to `PATH` if needed. To update, set `VERSION` to the new tag, repeat the
download and checksum steps, extract it, then replace only the executable:

```sh
install -m 0755 orza "$HOME/.local/bin/orza"
```

To remove the executable without deleting catalog data or remembered credentials:

```sh
rm -- "$HOME/.local/bin/orza"
```

### macOS

```sh
VERSION=v0.1.0
ARCH=arm64 # use amd64 on an Intel Mac
ASSET="orza_${VERSION}_darwin_${ARCH}.tar.gz"
CHECKSUMS="orza_${VERSION}_checksums.txt"
BASE="https://github.com/pluque01/orza/releases/download/${VERSION}"
curl -fLO "$BASE/$ASSET"
curl -fLO "$BASE/$CHECKSUMS"
grep "  $ASSET$" "$CHECKSUMS" | shasum -a 256 --check -
tar -xzf "$ASSET"
install -d "$HOME/.local/bin"
install -m 0755 orza "$HOME/.local/bin/orza"
"$HOME/.local/bin/orza" --version
"$HOME/.local/bin/orza" --help
```

The initial release is not code-signed or notarized. Review the release and checksum before deciding
whether to allow it under local macOS policy. To update after downloading, checking, and extracting a
new `VERSION`, replace only the executable; to remove it, remove only that executable:

```sh
install -m 0755 orza "$HOME/.local/bin/orza"
```

To remove it:

```sh
rm -- "$HOME/.local/bin/orza"
```

### Windows PowerShell

```powershell
$Version = "v0.1.0"
$Arch = "amd64" # use arm64 on Windows arm64
$Asset = "orza_${Version}_windows_${Arch}.zip"
$Checksums = "orza_${Version}_checksums.txt"
$Base = "https://github.com/pluque01/orza/releases/download/$Version"
Invoke-WebRequest "$Base/$Asset" -OutFile $Asset
Invoke-WebRequest "$Base/$Checksums" -OutFile $Checksums
$Line = Select-String -Path $Checksums -Pattern ("  " + [regex]::Escape($Asset) + "$") | Select-Object -ExpandProperty Line
if (-not $Line) { throw "Checksum entry not found for $Asset" }
$Expected = ($Line -split '\s+')[0].ToLowerInvariant()
$Actual = (Get-FileHash -Algorithm SHA256 $Asset).Hash.ToLowerInvariant()
if ($Actual -ne $Expected) { throw "SHA-256 mismatch for $Asset" }
Expand-Archive -Force $Asset .\orza-release
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\Orza"
New-Item -ItemType Directory -Force $InstallDir | Out-Null
Copy-Item -Force .\orza-release\orza.exe (Join-Path $InstallDir "orza.exe")
& (Join-Path $InstallDir "orza.exe") --version
& (Join-Path $InstallDir "orza.exe") --help
```

Add `%LOCALAPPDATA%\Programs\Orza` to the user `PATH` if desired. To update, download and verify a
new `$Version`, expand it, and replace only the executable. To remove only the executable:

```powershell
$InstallDir = Join-Path $env:LOCALAPPDATA "Programs\Orza"
Copy-Item -Force .\orza-release\orza.exe (Join-Path $InstallDir "orza.exe")
```

To remove it:

```powershell
Remove-Item (Join-Path $env:LOCALAPPDATA "Programs\Orza\orza.exe")
```

These SHA-256 checks detect corruption or bytes that differ from the published manifest. They
**do not authenticate the publisher or artifact by themselves** because release signing, macOS
notarization, and Windows code signing are outside the initial release scope.

Uninstalling by the commands above intentionally leaves the [catalog](#data-concurrency-and-recovery)
and OS credential-store entries in place. Do not delete catalog directories as part of an executable
update or removal. Delete remembered connections through Orza before uninstalling if those native
credential entries should also be removed.

## Build From Source

Nix with flakes enabled is the reproducible development and CI interface on Linux and macOS:

```sh
nix develop
nix flake check --no-update-lock-file --keep-going -L
nix build
./result/bin/orza --version
```

Install or remove the native Nix package with `nix profile install '.#default'` or
`nix profile remove orza`. The flake exposes native `packages.<system>.default` outputs for
`x86_64-linux`, `aarch64-linux`, `x86_64-darwin`, and `aarch64-darwin`, plus `windows-amd64` and
`windows-arm64` cross outputs. Nix does not run natively on Windows.

Without Nix, the module currently declares Go 1.26 and `toolchain go1.26.6`:

```sh
go test ./...
go install ./cmd/orza
```

Source builds report a development/static version. In contrast, stable release automation uses
**exactly Go 1.26.6** with `GOTOOLCHAIN=local`, vendored dependencies, path trimming, VCS metadata,
and the release version injected into all six executables.

On macOS, use a matching native Darwin builder with cgo enabled so the binary links the real Keychain
backend: `CGO_ENABLED=1 go install ./cmd/orza`. This normally requires Xcode Command Line Tools. A
Darwin build with cgo disabled uses the unavailable credential-store fallback and cannot remember
passwords in Keychain. Linux uses Secret Service over D-Bus; Windows uses Credential Manager.

The release gate proves that all six targets compile: Linux/Windows are built on Linux with cgo
disabled, while Darwin amd64/arm64 use matching native macOS runners with cgo enabled. It does not
execute all target binaries and therefore proves source compatibility, not native terminal, SSH,
filesystem, agent, or credential-store behavior.

Stable publication is triggered only by pushing an exact `vMAJOR.MINOR.PATCH` tag, without leading
zeroes, whose commit is already in `main` and has passed the required CI gate:

```sh
git tag v0.1.0
git push origin v0.1.0
```

## CLI

Run `orza --help`, `orza connection --help`, `orza folder --help`, or `orza tunnel --help`
for generated help.
Global options are:

| Option | Behavior |
|---|---|
| `--json` | Emit one machine-readable response for a catalog command or a structured pre-active `connect` failure. Interactive `connect` success remains a terminal stream. It cannot be used with `exec` or `tunnel`. |
| `--no-color` | Disable color. A non-empty `NO_COLOR` environment variable does the same. |
| `--help` | Print help without starting the TUI. |
| `--version` or `version` | Print the version. |

Logical paths are absolute and case-sensitive on every OS. `/` is the catalog root. Names cannot be
empty or contain `/`, NUL, or control characters. A selector beginning with `/` is a path; otherwise
it must be the canonical 32-character hexadecimal ID printed by `show` or `list`.

### Synthetic quickstart

These examples contain only non-secret connection metadata. Passwords and private-key passphrases are
always requested interactively without echo; do not put them in commands, environment variables,
shell history, scripts, or issue reports.

This local-only walkthrough uses the reserved `.invalid` domain and makes no network connection:

```sh
orza --version
orza --help
orza folder create /quickstart
orza folder create /quickstart/lab
orza connection create /quickstart/lab/shell \
  --host shell.example.invalid --port 22 --user operator --auth agent
orza connection show /quickstart/lab/shell
orza connection list /quickstart/lab
orza folder list /quickstart
orza connection update /quickstart/lab/shell --host shell-2.example.invalid
orza connection move /quickstart/lab/shell /quickstart
orza connection show /quickstart/shell
```

Run `orza` in an interactive terminal to open the TUI. Use Up/Down or `j`/`k` to select,
Left/Right or `h`/`l` to collapse/expand, `Tab` to change region, `?` for help, and `Esc` to cancel or
go back. Do not confirm `c` (connect) for the synthetic host. Press `q` from the browser or `Ctrl-C`
from any focus owner to request a safe exit; unsaved forms offer Save, Discard, or Cancel rather than
silently losing state. Remove the walkthrough records when finished:

```sh
orza folder delete /quickstart --recursive --yes
```

For key authentication, the catalog stores the path, not private-key contents:

```sh
orza connection create /work/database \
  --host database.example.invalid --user deploy --auth key \
  --identity-file ~/.ssh/id_ed25519
```

For password authentication, omit `--remember-password` to keep the password session-only and enter
it when connecting:

```sh
orza connection create /work/legacy \
  --host legacy.example.invalid --user deploy --auth password
orza connect /work/legacy
```

The current SSH session setup requires a non-empty saved user. Catalog creation accepts an omitted
`--user`, but such a connection cannot currently connect; provide `--user` explicitly.

### Connection commands

```text
orza connection create PATH --host HOST [--port PORT] [--user USER]
  --auth agent|key|password [--identity-file PATH] [--remember-password]
orza connection list [FOLDER_PATH_OR_ID]
orza connection show PATH_OR_ID
orza connection update PATH_OR_ID [--name NAME] [--host HOST] [--port PORT]
  [--user USER|--clear-user] [--auth agent|key|password]
  [--identity-file PATH|--clear-identity-file]
  [--remember-password|--forget-password] [--if-revision REVISION]
orza connection move PATH_OR_ID DESTINATION_FOLDER [--if-revision REVISION]
orza connection delete PATH_OR_ID [--if-revision REVISION] [--yes]
orza connection forget-host-key PATH_OR_ID [--if-revision REVISION]
orza connect PATH_OR_ID
orza exec PATH_OR_ID [--timeout DURATION] -- COMMAND
```

The default port is 22. `--identity-file` is required only for `key`. `--remember-password` is valid
only for password authentication and asks for explicit confirmation before reading the password.
Without `--yes`, deletion requires an interactive confirmation and identifies the target. A
non-interactive deletion must deliberately use `--yes`.

`connection forget-host-key` is non-interactive so it can be used in automation. It removes only
Orza's remembered trust for the connection's host and port; a repeated command reports that no
app-owned key remains. It does not modify `~/.ssh/known_hosts` or `~/.ssh/known_hosts2`.

### Remote command execution

`exec` runs one shell-text command on a saved connection without opening the TUI, allocating a PTY,
changing terminal mode, or prompting. Command text after `--` is passed unchanged to the remote SSH
endpoint, so quote it for the local shell and use remote pipes or redirections normally:

```sh
printf 'input\n' | orza exec /work/host -- 'cat; printf "diagnostic\n" >&2'
orza exec /work/host --timeout 30s -- 'long-running-command'
```

The caller's standard input is forwarded and stdout/stderr stream directly to their matching local
channels. The default timeout is five minutes and `--timeout` must be positive. A remote exit status
from 1 through 255 is returned unchanged. Local failures use the documented management statuses.
`exec` never approves or persists an unknown or changed host identity and never prompts for passwords
or key passphrases; approve trust through the interactive flow and use an SSH agent, unencrypted key,
or remembered password for unattended use.

### Foreground tunnels

`tunnel` runs one session-only TCP tunnel on a saved connection in the foreground, without a shell,
remote command, or PTY. It does not forward stdin. Use `orza tunnel MODE --help` for mode-specific help.

```text
orza tunnel local PATH_OR_ID --listen-port PORT --destination HOST:PORT
  [--listen-address IP] [--non-interactive] [--acknowledge-exposure]
orza tunnel remote PATH_OR_ID --listen-port PORT --destination HOST:PORT
  [--listen-address IP] [--non-interactive] [--acknowledge-exposure]
orza tunnel dynamic PATH_OR_ID --listen-port PORT
  [--listen-address IP] [--non-interactive] [--acknowledge-exposure]
```

| Mode | Listening machine | Destination and DNS |
|---|---|---|
| `local` | This computer | The saved SSH host reaches the fixed destination and resolves its hostname. |
| `remote` | Saved SSH host, subject to server policy | This computer reaches the fixed destination and resolves its hostname. |
| `dynamic` | This computer | SOCKS5 clients choose destinations; domain names sent to the proxy are resolved by the saved SSH host, not locally. |

`--listen-port` is required and accepts decimal ports from 1 through 65535; port zero is not supported.
`--listen-address` defaults to `127.0.0.1` and accepts an IP literal or `localhost` (normalized to IPv4
loopback). Use `::1` explicitly for IPv6 loopback. Local/Remote require a nonempty destination hostname
or IP and numeric port; bracket IPv6 destinations, for example `'[::1]:8080'`. Dynamic rejects
`--destination` and supports only SOCKS5 CONNECT with no-authentication, IPv4, domain, or IPv6 addresses.
It does not configure proxy clients for you; clients must send domain names to the proxy to avoid
client-side DNS resolution.

These examples make real SSH connections. Replace `/work/host` with an existing saved connection and
use destinations you are authorized to access:

```sh
orza tunnel local /work/host --listen-port 15432 --destination db.internal:5432
orza tunnel remote /work/host --listen-port 18080 --destination 127.0.0.1:8080
orza tunnel dynamic /work/host --listen-port 1080
orza tunnel local /work/host --listen-address ::1 --listen-port 18080 \
  --destination '[::1]:8080'
orza tunnel local /work/host --listen-port 15432 --destination db.internal:5432 \
  --non-interactive
```

Without `--non-interactive`, stdin and stdout must be terminals. Orza shows the captured catalog path,
SSH endpoint, mode, listening machine/endpoint, and applicable destination/reaching machine, then asks
for explicit confirmation with cancel as the default. Host trust is a separate reject/once/persist
decision; passwords and key passphrases use the existing no-echo input flow.

`--non-interactive` is mandatory without a terminal and may also be chosen explicitly in a terminal.
It never prompts or approves unknown/changed host identities. Use an agent, unencrypted key, or existing
remembered password; missing secrets and unavailable credentials fail. Revoked host identities always
fail. This policy is selected before bootstrap credential recovery: Linux does not unlock or run native
credential prompts, macOS uses fail-without-UI access, and Windows uses generic no-UI credential APIs.
Recovery is not skipped or falsely completed; inaccessible pending work fails startup under the existing
catalog policy. Suppressing native dialogs does not promise hard deadlines for synchronous OS calls.
Invalid tunnel options, unsupported `--json`, and missing explicit no-terminal policy are rejected before
bootstrap recovery can mutate state or acquire credentials.

Loopback is the safe default. Non-loopback listeners require separate exposure acknowledgement in the
interactive flow, or `--acknowledge-exposure` in unattended mode. The flag supplies exposure consent only:
it neither approves host trust nor skips interactive target confirmation. Other machines may reach such
listeners. Dynamic clients are **not authenticated**, even though the SSH transport is authenticated;
exposing it grants unauthenticated proxy access through the saved host. For example, this deliberately
exposes a proxy and should be used only on a controlled network with appropriate access controls:

```sh
orza tunnel dynamic /work/host --listen-address 0.0.0.0 --listen-port 1080 \
  --non-interactive --acknowledge-exposure
```

Remote readiness reports a **Requested listener**, never a verified listening address, and warns:
`Warning: Actual remote listening scope is unverified and depends on server configuration.` This applies
even to loopback requests. Server forwarding policy controls acceptance and actual exposure; settings such
as `AllowTcpForwarding`, `GatewayPorts`, `PermitListen`, and `PermitOpen` may restrict or alter forwarding.
Orza does not inspect or change those settings, infer precise policy from a generic refusal, or fall back
to another address/port. Readiness means the transport and listener request succeeded, not that the
destination is healthy. Remote listener release after transport loss depends on server detection and is
not verified by Orza.

All controlled progress, readiness, warnings, and failures go to stderr; stdout remains empty during
tunnel execution. `--json` is unsupported and there is no JSON event stream. Diagnostics are safe,
coalesced/rate-limited summaries, not raw library errors, payloads, secrets, or dynamic destination history.
The process remains foreground until interruption or tunnel-level failure; ordinary destination or
proxy-client failures do not stop it. `Ctrl-C` stops the listener and clients without a quit confirmation.
Handled termination joins cleanup before exit; `SIGKILL` cannot guarantee cleanup.

Tunnel results use existing management statuses: 1 for internal failure, 2 for invalid usage/endpoints or
missing exposure consent, 3 for a missing connection, 4 for a captured revision conflict, 5 for canceled
confirmation/input or owner cancellation without a signal, 6 for trust/authentication/credential failure,
7 for catalog or bootstrap recovery failure, and 10 for binding, forwarding refusal, or transport failure.
Controlled non-signal owner shutdown succeeds with 0; SIGINT/SIGTERM retain 130/143 after cleanup. No remote
command exit status is involved. Direct `orza connect` continues to propagate remote shell statuses
unchanged; [TUI shells](#forwarding-and-shells) instead return to the browser.

#### Limits and lifetime

- A TUI session admits at most 16 live tunnels, including Starting and Stopping; a CLI invocation owns one.
  Each tunnel uses a separate SSH transport, so stopping or failing one does not stop another.
- Each tunnel admits 64 client-work slots, including negotiation, pending destination opens, established
  copies, and unresolved teardown workers. Remote forwarding permits one additional serialized overflow
  rejection/cleanup worker. At capacity, new work is rejected rather than evicting a live tunnel or client.
- Destination establishment has a 10-second per-client deadline. SOCKS negotiation has a separate
  10-second socket deadline, cleared for established traffic. Slots remain owned until workers actually
  finish; late successful opens after cancellation are closed.
- An expired open or client/protocol cleanup that cannot converge within a further 1 second fails and
  force-closes that unhealthy tunnel transport. Ordinary destination refusal is not a tunnel failure.
- Forwarding shutdown arms an independent watchdog before graceful protocol cleanup; it forces raw SSH
  socket closure within 250 ms if cleanup has not converged. This is a forced-close fallback, not a
  total shutdown deadline. Local resources and owned workers are joined before final state publication.
- Healthy tunnels and established traffic have no idle timeout. Startup has no fixed deadline: a silent
  handshake or remote-listener reply can remain Starting until explicit cancellation. There is no tunnel
  lifetime/startup-timeout flag.
- The session retains at most 32 Stopped/Failed snapshots and one latest safe diagnostic per tunnel,
  bounded to 256 Unicode code points. Older terminal-state entries may be evicted; there is no client
  event queue or destination history.

Forwarding assumes a trusted SSH server. The SSH library accepts remote channels before application
admission and may transiently buffer protocol work. These application limits are **not a strict hard cap
on hostile-server library/multiplexer internals**. Traffic is streamed in both directions with bounded
copy buffers; a clean half-close allows reverse traffic to finish where supported.

Tunnels are not saved profiles or catalog records. There is no background/daemon mode, automatic reconnect,
global or cross-process tunnel list, arbitrary SSH flags, proxy authentication/TLS, destination ACL,
payload inspection, SOCKS BIND, or UDP ASSOCIATE. A separate TUI/CLI process cannot list or manage the
tunnels owned by this process.

### Folder commands

```text
orza folder create PATH
orza folder list [PATH_OR_ID]
orza folder show PATH_OR_ID
orza folder rename PATH_OR_ID NAME [--if-revision REVISION]
orza folder move PATH_OR_ID DESTINATION_FOLDER [--if-revision REVISION]
orza folder delete PATH_OR_ID [--recursive] [--if-revision REVISION] [--yes]
```

`folder list` shows direct child folders and connections, with folders first. `connection list` shows
only direct child connections. Parent folders must already exist. The root can be listed and shown but
cannot be renamed, moved, or deleted. Deleting a non-empty folder requires `--recursive`; the
confirmation reports the numbers of folders, connections, and remembered credentials affected.

### JSON, IDs, and revisions

Use JSON for automation:

```sh
orza --json connection show /work/bastion
orza --json folder list /work
```

A successful catalog operation writes exactly one JSON object to stdout:

```json
{
  "ok": true,
  "data": {
    "id": "0123456789abcdef0123456789abcdef",
    "kind": "connection",
    "name": "bastion",
    "path": "/work/bastion",
    "revision": 3,
    "host": "shell-2.example.invalid",
    "port": 22,
    "user": "deploy",
    "auth": "agent",
    "createdAt": "2026-07-29T10:00:00Z",
    "updatedAt": "2026-07-29T10:05:00Z"
  },
  "catalogRevision": 8
}
```

Optional fields such as `identityFile` are present only when applicable. Passwords, passphrases,
private-key contents, credential references, and session contents are never projected. A JSON failure
is one object on stderr and has `ok: false` plus `error.code`, `error.message`, and an optional
`error.target`.

`exec` and `tunnel` reject `--json`; the failure envelope above describes commands that support JSON.

Each item has a stable ID and an item `revision`. The independent `catalogRevision` changes whenever a
business transaction commits. For compare-and-swap automation, read the item revision and send it
back with `--if-revision`:

```sh
revision=3
orza connection update /work/bastion \
  --host shell-3.example.invalid --if-revision "$revision"
```

A stale item revision, conflicting name, or changed recursively confirmed scope exits with status 4
and does not overwrite the committed change. If `--if-revision` is omitted, the command resolves and
updates the latest version within one transaction.

| Exit status | Meaning |
|---|---|
| 0 | Catalog operation succeeded, or the remote shell exited with 0. |
| 2 | Invalid usage or validation. |
| 3 | Item not found. |
| 4 | Revision, name, or confirmed-scope conflict. |
| 5 | User canceled. |
| 6 | Host trust, authentication, or credential-store failure. |
| 7 | Catalog unavailable, incompatible, corrupt, or not writable. |
| 10 | Local SSH transport or protocol failure without a remote status. |

For direct `connect` or `exec`, a remote non-zero status from 1 through 255 is propagated as the
Orza process status, even when the number overlaps a management status. TUI shells return to the browser
with an outcome notice instead of determining the eventual TUI exit status.

### SSH startup diagnostics

Failures after target confirmation but before an SSH session becomes active use stable diagnostic
identifiers. Categories are `timeout`, `authentication_denied`, `connection_refused`, `host_not_found`,
`network_unreachable`, `host_trust`, `credential_unavailable`, `ssh_negotiation`, `canceled`, and
`unexpected`. Stages are `target_resolution`, `network_connection`, `host_trust`, `ssh_negotiation`,
`credential`, `authentication`, `session_setup`, `local_terminal`, and `unknown`. Here,
`target_resolution` means network name resolution, not catalog lookup. These identifiers are stable within
the current major version; controlled human summaries and recommendations may be refined.

Human CLI diagnostics are written to stderr and include a summary, captured target, captured `host:port`,
category, stage, recommendation, and optional safe detail. `orza --json connect SELECTOR` writes exactly
one pre-active failure object to stderr with this schema:

| Field | Presence |
|---|---|
| `ok` | Always `false`. |
| `error.code`, `error.message` | Existing broad error contract. |
| `error.target` | Captured catalog path, present for a startup diagnostic. |
| `error.endpoint` | Optional captured `host:port`; never includes a username, resolved address, credential reference, or identity path. |
| `error.category`, `error.stage`, `error.recommendation` | Present for an SSH startup diagnostic. |
| `error.technicalDetail` | Optional; omitted rather than set to null when no allowlisted detail exists. |

stdout remains the interactive stream: it can already contain the connection notice or host-trust prompt,
and becomes the remote session stream after startup. A failure object never crosses to stdout, and an
interactive session does not emit a JSON success envelope. Catalog not-found/conflict errors before startup,
remote statuses after activation, and terminating signal statuses remain outside the startup schema.

The categories add precision without changing broad compatibility: `canceled` remains `canceled`/exit 5;
`host_trust`, `authentication_denied`, and `credential_unavailable` remain `security_failure`/exit 6; all
other startup categories remain `transport_failure`/exit 10. Remote statuses 1 through 255 and conventional
signal statuses 130/143 retain their existing behavior.

Technical detail is not general error text. It is selected only from fixed application values for timeout,
authentication rejection, connection refusal, name resolution, unreachable network, enumerated host-trust
state, credential source, SSH setup step, or cancellation. It is normalized to one line, strips ANSI,
control, and bidirectional-control characters, and is limited to 256 Unicode code points. Raw operating
system, resolver, SSH library, server, wrapped-cause, password, passphrase, private-key, credential-reference,
and session text is never authorized for diagnostic display. Diagnostics exist only for the current attempt:
showing, expanding, closing, editing from, or retrying one creates no diagnostic persistence, history,
telemetry, or application log.

Startup classification and presentation target Windows, Linux, and macOS on amd64 and arm64. Classification
uses typed platform errors rather than localized message matching. Builds and cross-builds establish source
compatibility only; native terminal, network, and operating-system integration still require validation on
each target. At 80x24 the complete failure view and controls are visible. Reduced sizes prioritize category,
captured target, stage/recommendation, and recovery controls; at extremely small sizes labels are compacted.
`--no-color` and `NO_COLOR` preserve the same information through text labels and keys without ANSI color.

## TUI

Start the TUI by running `orza` with stdin and stdout attached to a terminal. With no command in a
pipeline or other non-interactive context, `orza` prints help and exits with status 2 instead of
drawing terminal control sequences.

### Terminal support and layout

The supported interface is a VT-capable terminal on Linux, macOS, or Windows that reports printable
keys, arrows, Tab, Escape, F1, F2, and Ctrl-C. The terminal must provide normal VT screen, keyboard,
resize, and bracketed-paste behavior; native operating-system integration still needs testing on each
platform. Startup verifies Windows VT input and output console modes directly. Unix terminal devices do
not expose a reliable VT protocol query, so startup verifies only that stdin and stdout are terminals;
on Linux and macOS, using a VT-capable emulator remains an explicit support prerequisite rather than a
runtime guarantee. No `TERM` value is treated as proof of capability. If Shift-Tab cannot be
distinguished, the visible, unmodified `F2 Previous` binding provides the same form traversal. `F1 Help`
remains available while an editable field owns printable input. A reliably detected missing VT
capability with no adapter-provided safe fallback is a startup error after terminal restoration.

The TUI is keyboard-only. Mouse navigation, activation, selection, and scrolling are not implemented
or required. Terminal-native text selection may still be provided by the emulator, but the application
does not consume mouse events or read the operating-system clipboard.

Every normal browser frame has three named panels and a borderless action legend:

- **Tree** shows the catalog hierarchy and the one selected row.
- **Details** shows the selected folder/connection or hosts an active connection form. Folder details
  list direct child connections only, not connections in descendant folders.
- **Tunnels** permanently shows session-owned tunnels, including an empty state and active count,
  independently of catalog selection or filtering.

The bottom legend shows available keys for the current selection and focus owner in at most three rows;
there is no titled or bordered Actions panel. Connection create/edit screens use their own Tree/Details
and control layout; forwarding drafts remain in the browser Details panel with Tunnels visible.

Panel titles use plain text such as `Details`; the existing `[*]` and `[ ]` markers identify active
and inactive regions. Content types use unbracketed labels such as `Connection` and `Folder`; Details
keeps its stable region title while identifying selected content separately. When styling is available,
content-type labels are bold without a foreground or background color; structured descriptive labels use
secondary emphasis while values remain primary.

Read-only identity rows align values in one shared column when the local panel leaves at least eight
display cells for values. Below that threshold, every label is followed by its indented value on the
next line; fields are not hidden to preserve alignment. Connection forms instead keep each label and
bounded control on one compact row, leaving the next line available for a validation error at 40x12.

At 80 columns or wider, Tree is left of vertically stacked Details and Tunnels, with the legend below.
Below 80 columns, Tree, Details, and Tunnels are stacked above the legend. The complete layout target is
**80x24**: Tree occupies 31x21, Details 48x11, and Tunnels 48x10, with a one-column gutter and three-row
legend. At **40x12**, each panel has three rows including borders, leaving one content row, followed by
the three-row legend. Larger stacked layouts share the panel height equally; wide layouts retain a
40%/60% left/right split and share the right-column height between Details and Tunnels. Changing focus
does not change panel positions or sizes; only resizing the terminal changes the browser geometry.
A wide or stacked frame below 80x24 is reduced and prioritizes the
active row or field, target identity, errors,
recovery/cancel/back/quit controls, and the primary action before secondary content. At sizes from
40x12, the regions remain usable with proportional scrollbars beside each overflowing container. Below **40x12**, the regions are replaced by an
undersized notice that permits only Help and safe Quit; selection, expansion, scroll, form values,
focus, modal, security input, and pending-operation state remain intact until resize.

Connection create/edit forms stay in Details. `Tab` moves forward and `Shift-Tab` or `F2` moves
backward; `Ctrl-S` saves, `Esc` cancels, `F1` opens Help, and `Ctrl-C` requests safe exit. Folder forms,
move selection, deletion and connection confirmation, unsaved-change decisions, Help, recoverable
errors, and SSH failures use one centered modal owner. A modal blocks the underlying Tree and Details;
Help requested from it is inline rather than a second stacked modal.

The controlled interface language is English only. Catalog names, paths, hosts, users, and safe backend
diagnostic values are displayed without translation. Run `NO_COLOR=1 orza` or `orza --no-color`
when color is unavailable. Color adds emphasis but carries no meaning by itself. No-color mode removes
ANSI styling while retaining the same title, content-type label, values, and textual semantics:
`[*]`/`[ ]` mark active/inactive regions, `>` marks selection or focus, `!` marks invalid input, `*`
marks a primary control, and `[/]`, `[+]`, `[-]`, `[ssh]`, `Warning:`, `Error:`, the `│`/`█` track and
thumb, and `…` preserve node, severity, overflow position, and truncation meaning. Scrollbars are
informational and remain keyboard-only; lower-priority legend actions remain discoverable in contextual
Help rather than making the legend scrollable.

| Keys | Action |
|---|---|
| Up/Down or `j`/`k` | Move through visible tree rows without wrapping. |
| Left/Right or `h`/`l` | Collapse/go to parent or expand/go to first child. |
| `g` / `G` | Select the first/root or last visible row. |
| `Enter` / Space on folder | Toggle folder expansion. |
| `Esc` | Close help, cancel, or go back. |
| `?` | Toggle contextual help. |
| `f` / `n` | Create in the selected folder, or as a sibling of the selected connection. |
| `e` / `m` / `d` in catalog focus | Edit, move, or delete the selected item. |
| `c` in catalog focus | Show path and endpoint confirmation for the selected connection. |
| `p` in catalog focus | Open a forwarding draft for the selected connection; inert on folders/root. |
| `t` in the browser | Focus Tunnels without changing catalog or tunnel selection. |
| `r` in catalog focus | Reload the catalog; on an error it performs the offered retry/reload action. |
| `Tab` / `Shift-Tab` or `F2` | Cycle Tree/Details/Tunnels focus, or move through form controls. |
| Up/Down or `j`/`k` in Tunnels | Select tunnel entries without wrapping. |
| `Enter` in Tunnels | Inspect complete endpoints, warnings, state, and latest safe diagnostic. |
| `s` in Tunnels | Confirm stop for a selected Starting/Active tunnel. |
| `r` in Tunnels | Review the current saved connection and retry a selected Stopped/Failed tunnel. |
| `d` in Tunnels | Dismiss a selected Stopped/Failed entry; never stop a live tunnel. |
| `Esc` in Tunnels | Return to Tree without stopping anything. |
| `F2` | Move to the previous form control when Shift-Tab is unavailable. |
| `F1` | Open Help while an editable field owns printable keys. |
| Left / Right | Cycle the visible `Agent`, `Key`, and `Password` method selector while Method is focused. |
| Space | Toggle password remembering while that control is focused. |
| `Ctrl-S` | Save a catalog form, or validate a forwarding draft and open start confirmation. |
| `y` | Explicitly confirm a destructive or connection action. |
| `s` / `d` / `Esc` in Unsaved Changes | Save, Discard, or Cancel. Save exits only after commit and cleanup; Discard exits without persisting; Cancel restores the form. |
| `q` | Request Quit from a browser or other non-text owner where it is offered. |
| `Ctrl+C` | Request safe Quit, including while a form owns text input. A dirty form follows the Save/Discard/Cancel path. |

On an SSH startup failure, the modal owns input and uses a contextual keymap: `d` shows or hides the
allowlisted detail without I/O, `r` resolves the captured connection ID and opens a fresh confirmation
without starting SSH, and `e` resolves that ID and opens its current record for editing. `Esc` closes the
failure and returns to the captured node when it still exists; `q` quits from the stable failure modal.
Retry never starts network activity until the refreshed target is explicitly confirmed. These meanings do
not change the browser meanings of `d`, `r`, or `e` when no startup failure is open.

The browser is one preorder tree rooted at `/`. The root starts expanded and selected; other folders
start collapsed. Siblings place folders before connections and use case-sensitive binary name order,
then ID. Move pickers disable a folder's own subtree. Destructive dialogs start on cancel and show the
captured path and affected scope. Unsaved forms require confirmation before discard. Conflicts preserve
the external change and offer reload instead of merging or overwriting it.

Contextual create and move operations also pin the parent/source/destination revision and full path in
the repository transaction. Renaming or moving an ancestor while a form or credential prompt is open
therefore produces a conflict rather than creating or moving under a path the user did not confirm.

Before connecting, the TUI confirms the captured path and `host:port`; the application compares the
captured revision again before network I/O so a concurrent edit cannot redirect an accepted target.
Bubble Tea then yields the terminal to SSH. A failure before the remote shell becomes
active returns to a stable TUI error screen. Once active, the remote shell owns stdin/stdout/stderr;
terminal resize events are sent to its PTY, and local terminal state is restored when it ends.

### Forwarding And Shells

Select a saved connection in Tree/Details and press `p`. The Details draft starts in Local mode with
listen address `127.0.0.1`; provide a listen port and, for Local/Remote, destination host/port. Left/Right
cycles Local/Remote/Dynamic while Mode owns focus. Tab and Shift-Tab/F2 traverse controls, F1 opens Help,
Space toggles the separate external-access acknowledgement, and `Ctrl-S` or Enter on `[ Start forwarding ]`
validates before opening a cancel-default start confirmation. The form uses the same title, aligned
colonless labels, selector choices, and focus/error/primary markers as New connection. Mode or listener edits reset exposure consent. Dynamic hides fixed
destination fields. Validation/failure preserves settings;
Esc from a changed draft offers Discard/Cancel. No forwarding profile is saved.

Confirmation identifies the captured path, SSH endpoint, mode, requested listener, and applicable
destination, and retains the Remote scope warning even for loopback. The connection revision is checked
before network I/O, followed by the existing trust and secret gates. Only one startup/security interaction
owns input at a time; already active tunnels continue.
Starting, Active, Stopping, Stopped, and Failed are textual states. Activation returns browsing focus to
Tunnels with the new entry selected.

Tunnels are listed in creation order across hosts in this TUI session. Their selection is independent of
the catalog selection; overflow scrolls to keep the selected row visible. The empty panel says
`No tunnels; select host, p Forward`. At narrow widths, use `Enter` inspection for full endpoints and
warnings. Inspection provides the same applicable `s` Stop, `r` Retry, `d` Dismiss, and Esc Back controls.
Retry resolves the same saved ID's current record and opens a fresh draft for review/consent; it never
silently reconnects. Stopped/Failed entries remain inspectable until dismissal or bounded eviction.

While Tunnels owns focus, catalog edit/move/delete/connect/create keys do not act on catalog items;
`d` dismisses only a terminal-state tunnel. Editable fields, search, modals, and security input retain
exclusive ownership of printable keys, including pasted text. Quit with live Starting/Active/Stopping
tunnels lists their captured targets and asks for confirmation to close them all, with Cancel as default.
Handled process termination bypasses confirmation but still restores the terminal and joins cleanup.
Below 40x12, Help and safe Quit remain available while other input, trust approval, and secret submission
wait for resize.

Opening `c` from catalog focus keeps tunnels running on separate transports while Bubble Tea yields the
terminal to the shell. Forwarding and cleanup continue even when UI events are delayed. Normal or nonzero
shell completion, and shell-only transport failure, restore the browser, reconcile current tunnel state,
and show a safe shell outcome notice. The shell's status does not become the eventual result of an
unrelated TUI quit. This differs from direct `orza connect`, whose remote exit propagation is unchanged.
Root termination/cancellation instead stops all owned resources and exits after restoration and cleanup.

Host-trust, password, and private-key-passphrase input are specialized security owners, not generic
modals. They temporarily preempt application input while preserving the previous Tree, Details, form,
or modal focus owner, which is restored unchanged when the prompt closes. Below 40x12 the prompt remains
pending but cannot accept trust or secret input until resize. Resize, retry, reload, cancellation, and a
late asynchronous result cannot accept trust or submit a secret.

Trust input displays controlled target/fingerprint information and requires an explicit decision;
reject is the default. Secret bytes are owned first by the terminal no-echo reader and then temporarily
by authentication or the credential mutation operation, not by the rendered TUI model. Controlled
buffers are wiped after use. Password persistence occurs only after the explicit `Remember password`
choice and uses the operating-system credential store. Private-key passphrases are never persisted.
These protections assume the local account, terminal emulator, operating system, SSH agent, credential
store, and out-of-band fingerprint channel are trusted; a compromised endpoint can observe terminal
input/output outside the application's control. Process cleanup can restore the terminal and wipe
controlled buffers on normal cancellation and handled termination, but `SIGKILL`, host compromise,
terminal recording, and scrollback are outside that guarantee.

### Authentication Method Selector

Create and edit forms show `Agent`, `Key`, and `Password` simultaneously; a new connection starts with
`Agent` selected. While Method has focus, Left/Right cycles those choices and typing, deletion, and paste
do not change the selection. `Key` shows `Identity file`; `Password` shows `Remember password`, initially
unchecked. Switching methods retains those in-form drafts until save or cancel, but only the final method's
applicable configuration is validated or saved. A retained Remember choice never requests or stores a
password unless Password is selected when saving.

### Text And Secret Paste

Connection text fields (excluding the closed Method selector) and the folder-name field accept terminal-native bracketed paste. Paste is
inserted at the cursor or replaces a Shift+arrow selection as one operation. CR, LF, NEL, `U+2028`, and
`U+2029` are removed; other control characters reject the whole payload without changing value,
cursor, or selection. Normal fields are limited to 4,096 Unicode runes. The application disables the
Bubbles OS-clipboard binding and never reads the system clipboard directly.

Full shortcut isolation requires a terminal that honors bracketed paste and VT keyboard events. If an
emulator silently ignores bracketed-paste mode, its bytes are indistinguishable from typing; normal
typing and cancel remain available, but isolation cannot be guaranteed. Secret editing on Windows
requires the process standard console input handle and fails closed before raw mode on alternate
handles or non-native Ultraviolet fallback readers that cannot be canceled safely.

Remembered-password, SSH-password, and private-key-passphrase prompts use a dedicated no-echo editor,
show one mask per Unicode rune, and accept at most 4,096 bytes. The prompt owns the bytes until terminal
cleanup completes; authentication then owns them temporarily and wipes controlled buffers after use.
Only an explicit `Remember password` choice may persist a password in the operating-system credential
store. On interrupt/termination, terminal restoration completes before conventional exit 130/143;
`SIGKILL` cannot run process cleanup.

## SSH And Trust Model

Orza uses `golang.org/x/crypto/ssh`; it does not invoke an `ssh` executable or a shell.

- Host identity is verified before an agent, password, or private-key passphrase is requested.
- `~/.ssh/known_hosts` and `~/.ssh/known_hosts2`, when present as regular files, are read-only trust
  sources. Orza does not modify either file.
- Unknown or changed keys require `reject`, `once`, or `persist`; reject is the default. Verify the
  displayed SHA-256 fingerprint through a separate trusted channel before accepting it.
- `once` applies only to that attempt. `persist` stores the host, port, algorithm, public key, and
  fingerprint in `catalog.db`. A changed persisted key is revision-protected against concurrent
  replacement.
- A key marked revoked by standard `known_hosts` data is always rejected and cannot be overridden.
- Changed keys can indicate a rebuilt host, DNS/address error, or man-in-the-middle attack. Do not
  accept one merely to clear the warning.

Authentication methods are deliberately singular per connection:

- **Agent:** Linux and macOS use `SSH_AUTH_SOCK`; Windows uses the OpenSSH agent named pipe
  `\\.\pipe\openssh-ssh-agent`. Private keys remain in the agent.
- **Key:** the catalog stores an identity-file path. The file must be regular and no larger than 1 MiB.
  Encrypted keys allow up to three no-echo passphrase attempts; passphrases are never persisted.
- **Password:** a remembered password is read from the OS credential store. If none is remembered or
  the store is unavailable while connecting, it is requested without echo for that session.

Protect identity files with appropriate OS permissions. The current client validates file type and
size but does not repair permissive private-key permissions for you.

### Unsupported in v1

The built-in SSH client does not parse OpenSSH `ssh_config` or `~/.ssh/config`. Consequently, v1 does
not provide `Host` aliases, `Include`, `Match`, `ProxyJump`, `ProxyCommand`, configuration-derived
users/ports/keys, or other OpenSSH option resolution.

The following are also outside v1:

- FIDO/security-key and PKCS#11 providers, keyboard-interactive and GSSAPI authentication, agent
  forwarding, and automatic fallback across multiple authentication methods.
- Dedicated file transfer and SCP/SFTP. TCP local/remote/dynamic forwarding is supported as described
  under [Foreground tunnels](#foreground-tunnels), but persistent forwarding profiles, automatic reconnect,
  cross-process management, UDP forwarding, and SOCKS BIND/proxy authentication are not.
- Inventory import/export, synchronization between machines, shared/team catalogs, and remote server
  administration.

## Credential Stores

Remembering is opt-in and starts unchecked. Passwords are never stored in SQLite, files, environment
variables, or application-managed encryption. The catalog holds only an opaque credential reference,
namespaced to that catalog.

| Platform | Store and behavior |
|---|---|
| Windows | Generic current-user Windows Credential Manager entry. |
| macOS | Non-synchronizing generic-password entry in the login Keychain, accessible when unlocked; requires cgo. |
| Linux | Default Secret Service collection over an encrypted D-Bus session. |

Linux servers and containers often have no user session D-Bus or Secret Service provider. In that
headless case, do not use `--remember-password`; create a password connection without it and enter the
password on each `connect`. There is intentionally no plaintext file fallback. A locked store may
display its native unlock prompt. Canceling or failing that prompt is reported as a secure-store
failure, not silently treated as persisted success.

Changing a connection away from password authentication, using `--forget-password`, deleting a
connection, or recursively deleting its folder also deletes the associated native credential. If
deletion cannot be verified, Orza reports failure and retains durable recovery information rather
than claiming success. A failed remember operation can have committed non-secret connection changes
before the native store failure; inspect the connection with `show` before retrying.

## Data, Concurrency, And Recovery

`catalog.db` is a local SQLite database containing folders, non-secret connection metadata, app-owned
host trust, revisions, and durable credential-operation state.

| Platform | Catalog path |
|---|---|
| Linux | `$XDG_DATA_HOME/orza/catalog.db` when `XDG_DATA_HOME` is absolute; otherwise `~/.local/share/orza/catalog.db` |
| macOS | `~/Library/Application Support/orza/catalog.db` |
| Windows | `%LOCALAPPDATA%\orza\catalog.db` |

The path is not configurable in v1. Keep it on a local disk, not NFS/SMB, cloud-sync, or other
replicated storage. Windows explicitly rejects UNC paths and detected OneDrive locations.

On Linux and macOS, the application requires the `orza` directory to be owned by the current user
with mode `0700`, and `catalog.db` to be a regular current-user-owned file with mode `0600`. Symlinks in
the private path are rejected. Windows creates and verifies a protected DACL that grants access only
to the current user, and rejects reparse points. Startup fails rather than continuing with broader or
ambiguous access.

Multiple processes may read and manage the catalog. Writes use SQLite transactions, a bounded
five-second busy timeout, per-item optimistic revisions, and a global catalog revision. A stale edit
or a recursive scope changed after confirmation is rejected; it is never merged or applied as
last-write-wins. Reload, inspect the new revision, and deliberately repeat the operation. A lock that
cannot be acquired within the timeout is an error, not an implicit retry forever.

Credential changes use durable saga records because SQLite and OS credential stores cannot share a
transaction. Pending operations are recovered at process startup before normal commands run. If the
required native store is unavailable, startup recovery can fail until that store is available again;
restoring D-Bus/Secret Service, unlocking Keychain, or restoring Credential Manager access is safer
than deleting recovery records manually.

For backup or diagnosis:

1. Exit active Orza processes before copying `catalog.db`; do not copy a live database and journal
   independently.
2. Copy the database only to storage with equivalent private permissions. The backup contains
   sensitive host/user metadata and public host keys, but not passwords or private keys.
3. Remembered passwords remain in the OS credential store and are not included in a catalog backup.
   Moving only `catalog.db` to another machine therefore does not migrate credentials.
4. If startup reports an incompatible or corrupt catalog, preserve the original file for diagnosis
   and restore a known-good offline copy. Do not replace its application/schema IDs or edit SQLite
   tables by hand.
5. On Linux/macOS, repair accidental permission broadening while no process is running with
   `chmod 700` on the application directory and `chmod 600` on `catalog.db`, then verify ownership.
   On Windows, restore a current-user-only protected DACL rather than disabling the check.

## Security Notes

- Use disposable hosts and credentials for testing. Never paste production passwords, passphrases,
  private keys, credential-store records, or raw session output into commands, logs, bug reports, or
  JSON fixtures.
- Treat the catalog as sensitive even though it contains no passwords: it reveals hostnames, users,
  identity-file paths, organization, and trust history.
- Confirm every first-use or changed host fingerprint out of band. `persist` is a security decision,
  not a way to suppress an unfamiliar warning.
- `--yes` bypasses only the application's interactive deletion confirmation. Review the exact path,
  especially with `folder delete --recursive --yes`, before using it in automation.
- Do not run Orza with elevated privileges to work around file, agent, Keychain, Credential
  Manager, Secret Service, or socket permissions. Doing so selects a different home, catalog, and
  trust/credential context.
- Session output is streamed directly. It may contain secrets emitted by the remote system; protect
  terminal scrollback, recording, and surrounding automation accordingly.

## Tests

The current flake check builds and tests the native package and checks Go formatting:

```sh
nix flake check --no-update-lock-file --keep-going -L
```

Run native Go checks from `nix develop` (or a direct Go 1.26.6 environment):

```sh
go test ./...
go test -race ./...
go vet ./...
```

Optional release-analysis tools, when installed separately, can be run with:

```sh
staticcheck ./...
govulncheck ./...
```

Native validation is required for Windows console/DACL/Credential Manager/OpenSSH-agent behavior,
macOS terminal/Keychain behavior, and Linux terminal/mode/Secret Service/agent behavior. Run SSH
integration tests only against a controlled disposable server, never a production host.
