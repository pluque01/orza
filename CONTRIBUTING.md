# Contributing To Orza

## Local Environment

Use Nix with flakes enabled to enter the pinned development environment:

```sh
nix develop
```

Use only synthetic `.invalid` SSH hosts and disposable catalog data in development and tests.
Never use a real secret, credential, private key, passphrase, host fingerprint, production endpoint,
or personal catalog as a fixture.

## Required Checks

Run the same reproducible gate required for pull requests:

```sh
nix flake check --no-update-lock-file --keep-going -L
```

Changes to modules must leave `go.mod`, `go.sum`, and the Orza `vendorHash` in `flake.nix`
coherent. `vendor/` is ignored and must not be committed. Nix uses `buildGoModule` with
`proxyVendor = true`: the hash covers a local Go module proxy containing dependency sources,
including third-party license and notice files. Keep these files intact in the fetched sources.
Focused shell tests live in `scripts/*_test.sh` and use disposable repositories and mocked services.

### Updating Go dependencies

From the pinned shell, select explicit versions, then refresh the hash:

```sh
nix develop
GOTOOLCHAIN=local go get example.org/module@vX.Y.Z # substitute the reviewed module/version
bash scripts/update-go-dependencies.sh
nix build --no-update-lock-file --option sandbox true -L
nix flake check --no-update-lock-file --option sandbox true --keep-going -L
git diff -- go.mod go.sum flake.nix
test -z "$(git ls-files vendor)"
```

The updater tidies the manifests, fetches the dependency derivation with an empty expected hash
overridden **in memory**, accepts only an unambiguous Orza fixed-output hash mismatch, writes the
real SRI hash, then builds the dependency derivation with that hash. It never writes a placeholder.
Do not accept a hash from an unrelated fetch failure. Downloads and checksum-database verification
belong to this update/fixed-output acquisition phase. Required checks populate a fresh module cache
from the hash-verified local proxy, then run with `GOPROXY=off`, `GOSUMDB=off`, and
`GOTOOLCHAIN=local`; tidy must leave both manifests byte-for-byte unchanged.

Renovate's Go manager does not update this hash itself. The repository's `postUpgradeTasks` runs
`bash scripts/update-go-dependencies.sh` once per Go update branch after manifest changes, including
major updates, and retains `go.mod`, `go.sum`, and `flake.nix`. The scheduled self-hosted CLI uses
the pinned tools on PATH (`RENOVATE_BINARY_SOURCE=global`) and explicitly authorizes that exact
command through `RENOVATE_ALLOWED_COMMANDS`, alongside the existing vulndb lock command.
`RENOVATE_CUSTOM_ENV_VARIABLES` passes `GOTOOLCHAIN=local` to its filtered child environment;
setting only the parent process variable would not enforce it for Go artifact updates.
Only trusted scheduled runs receive the repository-scoped token (metadata read; Contents, Pull
requests, Issues, and Workflows read/write). PRs and manual discovery receive no write token.
The command executes repository code and evaluates its flake, so keep it restricted to trusted
Renovate branches and never enable it for arbitrary PR code. The token scope, weekly guard,
five-proposal limits, explicit approval, and ordinary CI remain required.
Manual local discovery can validate extraction/configuration but does not prove post-upgrade
execution; validate changed branch files and command execution in the scheduled run logs.

### Build guarantees

`flake.lock` fixes Nix inputs and toolchain; `go.mod` fixes module versions; `go.sum` authenticates
module content against reviewed checksums; `vendorHash` additionally fixes the complete proxy tree.
The ordinary Nix compilation/check derivations run in the sandbox without external network access.
These are distinct guarantees: bit-for-bit reproducibility across independent builds/platforms has
not been established. Non-Nix `go install`/`go test` uses Go's module checksums but lacks the Nix
dependency-tree hash, pinned native tools, and sandbox isolation.

## Pull Requests

Pull requests should address one approved specification or clearly stated maintenance objective,
include behavior-focused tests for the main path and principal failure path, and update user-facing
documentation in the same change. Describe security implications and manual checks that cannot be
automated. The stable `CI / required` check must pass before merge. Automatic dependency proposals
follow the same gate and are never merged automatically.

The sole maintainer may merge a validated change without an external approval. When a second
maintainer joins, governance must require one other-person approval and code-owner review for
sensitive paths.

## Sensitive Paths

Changes to `.github/`, `scripts/`, release and dependency configuration, Nix and Go supply-chain
inputs, vulnerability policy, security documentation, licensing, or dependency hashes require review
from the owner listed in `.github/CODEOWNERS`. Keep untrusted values quoted, permissions minimal,
diagnostics bounded, and secret values out of logs.

## Releases

Stable releases are created only by the release workflow from an exact `vMAJOR.MINOR.PATCH` tag on a
commit already integrated into `main` with required CI passing. Contributors must not create,
replace, or manually upload release assets. Release changes must preserve the six-platform archive
inventory, checksum manifest, draft-first publication, and the Go version pin in
`.github/release.env`.

All six release binaries use `release.nix` and the locked Nix toolchain. Native Darwin builds keep
cgo enabled for Keychain. Release source includes the credential-free checkout's Git metadata so
Go can stamp the genuine revision/time; the workflow checks the revision, clean VCS state, and exact
Go version. Build outputs are excluded from that source. Default development packages use clean
sources without Git metadata. Git data copied into the Nix store must contain no credentials.
