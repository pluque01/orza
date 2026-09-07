# Quality Gate Evidence

**Date**: 2026-07-31
**Environment**: Linux amd64, Go 1.26.5 through the pinned Nix development shell

| Gate | Command | Result |
|---|---|---|
| Formatting | `nix develop -c go fmt ./...` | PASS, no output after final cleanup |
| Complete tests | `nix develop -c go test ./...` | PASS, including `tests/integration` |
| Race detector | `nix develop -c go test -race ./...` | PASS |
| Vet | `nix develop -c go vet ./...` | PASS |
| Staticcheck | `nix develop -c staticcheck ./...` | PASS after removing five superseded TUI members |
| Vulnerabilities | `nix develop -c govulncheck ./...` | PASS, zero reachable vulnerabilities |
| Nix checks | `nix flake check` | PASS, all 13 current-system checks |
| Windows amd64 build | `nix build '.#windows-amd64'` | PASS |
| Windows arm64 build | `nix build '.#windows-arm64'` | PASS |
| Linux arm64 build | `nix build '.#linux-arm64'` | PASS |
| Security/regression review | Focused review of terminal reader, contextual CAS, paste, tree, and narrow views | PASS, no remaining high/medium findings |

`nix flake check` reported that native checks for `aarch64-linux`, `x86_64-darwin`, and
`aarch64-darwin` were omitted as incompatible with this x86_64-linux host. Cross-build success is not
recorded as native validation.

After review fixes, the complete tests, race detector, vet, staticcheck, govulncheck, Nix checks, and all
three cross-builds above were rerun successfully. Contextual create/move now compare direct revisions and
captured ancestor-sensitive paths atomically. Windows secret input positively certifies the pinned
Ultraviolet native cancel reader and rejects fallbacks before changing terminal modes.
