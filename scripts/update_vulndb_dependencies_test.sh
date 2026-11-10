#!/usr/bin/env bash
set -euo pipefail
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init
mkdir -p "$TEST_TMPDIR/source"
cd "$TEST_TMPDIR/source"
export TEST_UPDATE_MODE=success
stub_command nix <<'STUB'
case "$*" in
  'flake lock --update-input vulndb')
    printf 'updated lock\n' >flake.lock
    [ "$TEST_UPDATE_MODE" != lock-error ] ;;
  *overrideAttrs*)
    if [ "$TEST_UPDATE_MODE" = fetch-error ]; then
      printf 'network failed\n' >&2
    elif [ "$TEST_UPDATE_MODE" = unrelated-hash ]; then
      printf "error: hash mismatch in fixed-output derivation '/nix/store/test-other-go-modules.drv':\n  got: sha256-j/XFDaxgaKNsbqF+5OFeqKLIS7WXuA90DbCFCfSxChY=\n" >&2
    else
      printf "error: hash mismatch in fixed-output derivation '/nix/store/test-vulndb-indexer-unstable-go-modules.drv':\n  got: sha256-j/XFDaxgaKNsbqF+5OFeqKLIS7WXuA90DbCFCfSxChY=\n" >&2
    fi
    exit 1 ;;
  *) [ "$TEST_UPDATE_MODE" != verify-error ] ;;
esac
STUB
for mode in success lock-error fetch-error unrelated-hash verify-error; do
  export TEST_UPDATE_MODE=$mode
  printf 'vulndbVendorHash = "sha256-cHjJPluvGtuau9+qKTuT7PLfU1Iz56oG7eIYwHbdkY8=";\n' >flake.nix
  printf 'original lock\n' >flake.lock
  cp flake.nix original.nix
  cp flake.lock original.lock
  if output=$(bash "$script_dir/update-vulndb-dependencies.sh" 2>&1); then
    [ "$mode" = success ] || fail "accepted failed update: $mode"
    assert_contains "$(<flake.nix)" 'sha256-j/XFDaxgaKNsbqF+5OFeqKLIS7WXuA90DbCFCfSxChY='
    assert_eq 'updated lock' "$(<flake.lock)"
  else
    [ "$mode" != success ] || fail "$output"
    cmp flake.nix original.nix || fail "flake not restored after $mode"
    cmp flake.lock original.lock || fail "lock not restored after $mode"
  fi
done
printf 'PASS: vulndb hash update and rollback after lock, fetch, unrelated hash and validation failures\n'
