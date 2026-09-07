# Orza

[![CI](https://github.com/pluque01/orza/actions/workflows/ci.yml/badge.svg)](https://github.com/pluque01/orza/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/pluque01/orza)](https://github.com/pluque01/orza/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

A terminal-native SSH client with a TUI

Navigate remote systems from your terminal.

Orza is a local, terminal-first SSH connection manager. It keeps connections in its own
hierarchical catalog, exposes the same catalog through a CLI and a keyboard-driven TUI, and opens
interactive SSH sessions with host-key verification.

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

Run `orza --help`, `orza connection --help`, or `orza folder --help` for generated help.
Global options are:

| Option | Behavior |
|---|---|
| `--json` | Emit one machine-readable response for a catalog command or a structured pre-active `connect` failure. Interactive `connect` success remains a terminal stream. |
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
orza connect PATH_OR_ID
```

The default port is 22. `--identity-file` is required only for `key`. `--remember-password` is valid
only for password authentication and asks for explicit confirmation before reading the password.
Without `--yes`, deletion requires an interactive confirmation and identifies the target. A
non-interactive deletion must deliberately use `--yes`.

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

For an established SSH session, a remote non-zero status from 1 through 255 is propagated as the
Orza process status, even when the number overlaps a management status.

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

Every normal frame has three named regions:

- **Tree** shows the catalog hierarchy and the one selected row.
- **Details** shows the selected folder/connection or hosts an active connection form. Folder details
  list direct child connections only, not connections in descendant folders.
- **Actions** shows the exact available keys for the current selection and focus owner.

At 80 columns or wider, Tree is left of Details and Actions spans the bottom. Below 80 columns, Tree is
above Details and Actions remains at the bottom. The complete layout target is **80x24**. A wide or
stacked frame below 80x24 is reduced and prioritizes the active row or field, target identity, errors,
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
when color is unavailable. No-color mode removes ANSI styling but retains textual semantics: `[*]`/`[ ]`
mark active/inactive regions, `>` marks selection or focus, `!` marks invalid input, `*` marks a primary
control, and `[/]`, `[+]`, `[-]`, `[ssh]`, `Warning:`, `Error:`, the `│`/`█` track and thumb, and `…` preserve node,
severity, overflow position, and truncation meaning. Scrollbars are informational and remain keyboard-only;
when Actions omits lower-priority options it shows `Hidden actions — ? Help` instead of becoming scrollable.

| Keys | Action |
|---|---|
| Up/Down or `j`/`k` | Move through visible tree rows without wrapping. |
| Left/Right or `h`/`l` | Collapse/go to parent or expand/go to first child. |
| `g` / `G` | Select the first/root or last visible row. |
| `Enter` / Space on folder | Toggle folder expansion. |
| `Esc` | Close help, cancel, or go back. |
| `?` | Toggle contextual help. |
| `f` / `n` | Create in the selected folder, or as a sibling of the selected connection. |
| `e` / `m` / `d` | Edit, move, or delete the selected item. |
| `c` | Show path and endpoint confirmation for the selected connection. |
| `r` | Reload the catalog; on an error it performs the offered retry/reload action. |
| `Tab` / `Shift-Tab` | Switch Tree/Details focus, or move through form controls. |
| `F2` | Move to the previous form control when Shift-Tab is unavailable. |
| `F1` | Open Help while an editable field owns printable keys. |
| Left / Right | Cycle authentication methods while that field is focused. |
| Space | Toggle password remembering while that control is focused. |
| `Ctrl-S` | Save a form. |
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

### Text And Secret Paste

All connection text fields and the folder-name field accept terminal-native bracketed paste. Paste is
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
- Dedicated file transfer, SCP/SFTP, local/remote/dynamic tunnels, port forwarding, and non-interactive
  remote-command execution.
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
