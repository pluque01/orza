# Quality Gate Results

**Date:** 2026-09-07
**Overall status:** local reproducible checks and Git validation passed; hosted validation pending

## Passed

- `actionlint .github/workflows/*.yml`
- `shellcheck scripts/*.sh`
- Every focused `scripts/*_test.sh` automation suite
- `gofmt -d cmd internal tests`
- `go test ./...`
- `go test -race ./...`
- `go vet ./...`
- `staticcheck ./...`
- `go build ./...`
- Nix formatting, tests, race, vet, Staticcheck, build, automation, and native smoke checks
- Linux amd64/arm64 and Windows amd64/arm64 Nix release-output builds
- `gitleaks dir --redact --config .gitleaks.toml .` (no leaks found)
- Pinned vulnerability policy through the Nix flake
- Live vulnerability policy through `scripts/check-vulnerabilities.sh live`
- `scripts/check-vendor.sh` in `nix develop` after the initial commit
- `scripts/prepublish.sh --repository . --history new` in `nix develop`

The direct Go checks ran inside `nix develop`; the host environment does not expose `go` on `PATH`.
The reproducible environment reports `go version go1.26.6 linux/amd64`. The complete
`nix flake check --no-update-lock-file --keep-going -L` gate passes.

## Vulnerability Resolution

The Go toolchain was upgraded from 1.26.5 to 1.26.6. This resolves the reachable standard-library
findings previously reported by the live database: `GO-2026-5026`, `GO-2026-5942`,
`GO-2026-5972`, `GO-2026-6088`, `GO-2026-6089`, `GO-2026-6090`, `GO-2026-6091`, and
`GO-2026-6218`.

`GO-2026-5932` is emitted by `govulncheck` with only a module frame for `golang.org/x/crypto`:
there is no imported affected package or invoked symbol in its trace. The policy now preserves this
finding as visible, non-blocking information instead of misclassifying every module-only finding as
reachable. Package- and symbol-level traces continue to block unless an exact active exception exists.
A regression fixture covers this protocol behavior.

No vulnerability exception was added.

## Git Validation

- A new `main` history was initialized because no authoritative Git history was supplied.
- Root commit `badb0159f566ae580a26ce13048e5fdf40fe5631` was created with the authorized author identity.
- The committed dependency graph regenerates without changes, and the all-ref history scan passes.

## Not Run

- Hosted CI, native Darwin release builds, GitHub repository settings, and release publication require
  a connected private repository and authenticated GitHub access.

The vulnerability blocker and local publication gates are resolved. Tasks T051-T055 still require
private GitHub configuration, hosted checks, the participant study, repository-owner authorization,
and public release evidence specified by the task plan. No public visibility or release operation was
attempted.
