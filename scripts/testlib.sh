#!/usr/bin/env bash
set -euo pipefail

# Shared assertions for shell automation tests. Test scripts call testlib_init
# after enabling their preferred shell options.
testlib_init() {
  TEST_TMPDIR=$(mktemp -d "${TMPDIR:-/tmp}/orza-test.XXXXXX")
  TEST_BIN="$TEST_TMPDIR/bin"
  mkdir -p "$TEST_BIN"
  PATH="$TEST_BIN:$PATH"
  export TEST_TMPDIR TEST_BIN PATH
  trap 'rm -rf "$TEST_TMPDIR"' EXIT HUP INT TERM
}

fail() {
  printf 'FAIL: %s\n' "$*" >&2
  exit 1
}

assert_eq() {
  [ "$1" = "$2" ] || fail "expected <$1>, got <$2>"
}

assert_contains() {
  case $1 in
    *"$2"*) ;;
    *) fail "output did not contain expected text: $2" ;;
  esac
}

assert_not_contains() {
  case $1 in
    *"$2"*) fail "output contained redacted text" ;;
    *) ;;
  esac
}

assert_redacted() {
  assert_not_contains "$2" "$1"
}

# Writes an executable command stub from standard input.
stub_command() {
  local name=$1
  {
    printf '#!%s\n%s\n' "$BASH" 'set -eu'
    tee
  } >"$TEST_BIN/$name"
  chmod +x "$TEST_BIN/$name"
}

snapshot_tree() {
  local directory=$1
  (
    cd "$directory" || exit
    tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner -cf - .
  ) | sha256sum | cut -d ' ' -f 1
}

assert_tree_unchanged() {
  local expected=$1
  local directory=$2
  local actual
  actual=$(snapshot_tree "$directory")
  [ "$actual" = "$expected" ] || fail "unexpected filesystem side effect under $directory"
}
