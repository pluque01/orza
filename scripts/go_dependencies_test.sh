#!/usr/bin/env bash
set -euo pipefail
script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init
mkdir -p "$TEST_TMPDIR/source"
cd "$TEST_TMPDIR/source"
printf 'module example.invalid/test\n' >go.mod
printf 'original checksum\n' >go.sum
export TEST_MODE=clean
stub_command go <<'STUB'
[ "$GOPROXY" = off ] && [ "$GOSUMDB" = off ] && [ "$GOTOOLCHAIN" = local ] || exit 92
case "$*:$TEST_MODE" in
  'mod tidy:mod') printf 'changed\n' >>go.mod ;;
  'mod tidy:sum') printf 'changed\n' >>go.sum ;;
  'mod verify:corrupt') exit 1 ;;
esac
STUB
bash "$script_dir/check-go-dependencies.sh"
for mode in mod sum corrupt; do
  printf 'module example.invalid/test\n' >go.mod
  printf 'original checksum\n' >go.sum
  export TEST_MODE=$mode
  if bash "$script_dir/check-go-dependencies.sh" >/dev/null 2>&1; then
    fail "accepted inconsistent dependencies: $mode"
  fi
done
printf 'PASS: offline dependency integrity and inconsistent manifests\n'
