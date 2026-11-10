#!/usr/bin/env bash
set -euo pipefail

# Run from the repository root. Do not enter the updated development shell:
# its ORZA_VULNDB cannot be built until this hash has been refreshed.
export ORZA_UPDATE_SOURCE
ORZA_UPDATE_SOURCE=$(pwd -P)
# Nix interpolation, deliberately not shell interpolation.
# shellcheck disable=SC2016
expression='(builtins.getFlake ("path:" + builtins.getEnv "ORZA_UPDATE_SOURCE")).packages.${builtins.currentSystem}.vulndb-indexer'
temporary=$(mktemp -d)
cp flake.nix flake.lock "$temporary/"
completed=false
cleanup() {
  if [ "$completed" != true ]; then
    cp "$temporary/flake.nix" flake.nix
    cp "$temporary/flake.lock" flake.lock
  fi
  rm -rf "$temporary"
}
trap cleanup EXIT
trap 'exit 1' HUP INT TERM

nix flake lock --update-input vulndb
# Force a fresh dependency fetch even if the previous fixed output is cached.
if nix build --impure --no-link --no-update-lock-file --option sandbox true \
  --expr "($expression).goModules.overrideAttrs { outputHash = \"\"; outputHashAlgo = \"sha256\"; }" \
  >"$temporary/stdout" 2>"$temporary/log"; then
  printf 'vulndb: expected a fixed-output hash mismatch\n' >&2
  exit 1
fi
python3 - "$temporary/log" <<'PY'
import pathlib, re, sys
log = pathlib.Path(sys.argv[1]).read_text()
hashes = re.findall(r"got:\s+(sha256-[A-Za-z0-9+/]{43}=)", log)
if len(hashes) != 1 or not re.search(r"hash mismatch in fixed-output derivation '.*vulndb-indexer-.*go-modules", log):
    print(log, file=sys.stderr)
    sys.exit("vulndb: dependency fetch failed without an unambiguous indexer hash")
path = pathlib.Path("flake.nix")
updated, count = re.subn(r'vulndbVendorHash = "sha256-[A-Za-z0-9+/]{43}=";',
                         f'vulndbVendorHash = "{hashes[0]}";', path.read_text())
if count != 1:
    sys.exit("vulndb: expected exactly one vulndbVendorHash assignment")
path.write_text(updated)
print(f"vulndb: vulndbVendorHash = {hashes[0]}")
PY
if ! nix build --impure --no-link --no-update-lock-file --option sandbox true --expr "$expression" -L; then
  printf 'vulndb: indexer validation failed; restoring flake and lock\n' >&2
  exit 1
fi
completed=true
