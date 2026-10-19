#!/usr/bin/env bash
set -euo pipefail

# Run from the repository root, inside the pinned development environment.
export GOTOOLCHAIN=local
go mod tidy
export ORZA_UPDATE_SOURCE
ORZA_UPDATE_SOURCE=$(pwd -P)
# Nix interpolation, deliberately not shell interpolation.
# shellcheck disable=SC2016
expression='(builtins.getFlake ("path:" + builtins.getEnv "ORZA_UPDATE_SOURCE")).packages.${builtins.currentSystem}.default.goModules'
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
cp flake.nix "$temporary/flake.nix"
# Override the expected hash only in memory. Never write a placeholder to the flake.
if nix build --impure --no-link --no-update-lock-file --option sandbox true \
  --expr "($expression).overrideAttrs { outputHash = \"\"; outputHashAlgo = \"sha256\"; }" \
  >"$temporary/stdout" 2>"$temporary/log"; then
  printf 'dependencies: expected a fixed-output hash mismatch\n' >&2
  exit 1
fi
python3 - "$temporary/log" <<'PY'
import pathlib, re, sys
log = pathlib.Path(sys.argv[1]).read_text()
hashes = re.findall(r"got:\s+(sha256-[A-Za-z0-9+/]{43}=)", log)
if len(hashes) != 1 or not re.search(r"hash mismatch in fixed-output derivation '.*orza-.*go-modules", log):
    print(log, file=sys.stderr)
    sys.exit("dependencies: dependency fetch failed without an unambiguous Orza hash")
path = pathlib.Path("flake.nix")
original = path.read_text()
updated, count = re.subn(r'orzaVendorHash = "sha256-[A-Za-z0-9+/]{43}=";',
                         f'orzaVendorHash = "{hashes[0]}";', original)
if count != 1:
    sys.exit("dependencies: expected exactly one orzaVendorHash assignment")
path.write_text(updated)
print(f"dependencies: orzaVendorHash = {hashes[0]}")
PY
if ! nix build --impure --no-link --no-update-lock-file --option sandbox true --expr "$expression" -L; then
  cp "$temporary/flake.nix" flake.nix
  printf 'dependencies: hash validation failed; original flake restored\n' >&2
  exit 1
fi
