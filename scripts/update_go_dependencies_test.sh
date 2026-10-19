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
stub_command go <<'STUB'
[ "$GOTOOLCHAIN" = local ] && [ "$*" = 'mod tidy' ]
STUB
stub_command nix <<'STUB'
case "$*" in
  *overrideAttrs*)
    if [ "$TEST_UPDATE_MODE" = fetch-error ]; then
      printf 'network failed\n' >&2
    else
      printf "error: hash mismatch in fixed-output derivation '/nix/store/test-orza-linux-amd64-go-modules.drv':\n  got: sha256-j/XFDaxgaKNsbqF+5OFeqKLIS7WXuA90DbCFCfSxChY=\n" >&2
    fi
    exit 1 ;;
  *) [ "$TEST_UPDATE_MODE" != verify-error ] ;;
esac
STUB
for mode in success fetch-error verify-error; do
  export TEST_UPDATE_MODE=$mode
  # An existing real hash, deliberately wrong for this synthetic Orza fixture.
  printf 'orzaVendorHash = "sha256-cHjJPluvGtuau9+qKTuT7PLfU1Iz56oG7eIYwHbdkY8=";\n' >flake.nix
  if output=$(bash "$script_dir/update-go-dependencies.sh" 2>&1); then
    [ "$mode" = success ] || fail "accepted failed update: $mode"
    assert_contains "$(<flake.nix)" 'sha256-j/XFDaxgaKNsbqF+5OFeqKLIS7WXuA90DbCFCfSxChY='
  else
    [ "$mode" != success ] || fail "$output"
    assert_contains "$(<flake.nix)" 'sha256-cHjJPluvGtuau9+qKTuT7PLfU1Iz56oG7eIYwHbdkY8='
  fi
done
printf 'PASS: hash updater, fetch failure and failed verification rollback\n'
