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

Changes to modules must leave `go.mod`, `go.sum`, and `vendor/` coherent. Run `go mod tidy` and
`go mod vendor`, review every resulting change, and preserve all third-party license and notice files
under `vendor/`. Focused shell tests live in `scripts/*_test.sh` and must use disposable repositories
and mocked external services.

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
inputs, vulnerability policy, security documentation, licensing, or vendored source require review
from the owner listed in `.github/CODEOWNERS`. Keep untrusted values quoted, permissions minimal,
diagnostics bounded, and secret values out of logs.

## Releases

Stable releases are created only by the release workflow from an exact `vMAJOR.MINOR.PATCH` tag on a
commit already integrated into `main` with required CI passing. Contributors must not create,
replace, or manually upload release assets. Release changes must preserve the six-platform archive
inventory, checksum manifest, draft-first publication, and the Go version pin in
`.github/release.env`.
