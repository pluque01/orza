#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

state=$TEST_TMPDIR/state
mode_file=$TEST_TMPDIR/mode
arguments=$TEST_TMPDIR/git-arguments
export state mode_file arguments

stub_command go <<'EOF'
case "$*" in
  'mod tidy') printf 'tidied\n' >"$state" ;;
  'mod vendor') printf 'vendored\n' >"$state" ;;
  *) exit 90 ;;
esac
EOF

stub_command git <<'EOF'
printf '%s\n' "$*" >>"$arguments"
mode=$(<"$mode_file")
stage='initial'
[ ! -f "$state" ] || stage=$(<"$state")
case "$mode:$stage" in
  tidy:tidied) printf ' M go.mod\n' ;;
  tracked:vendored) printf ' M vendor/modules.txt\n' ;;
  untracked:vendored) printf '?? vendor/example/new.go\n' ;;
esac
EOF

export ORZA_GO_BIN=$TEST_BIN/go ORZA_GIT_BIN=$TEST_BIN/git

run_case() {
  local mode=$1 expected=$2
  printf '%s\n' "$mode" >"$mode_file"
  rm -f "$state"
  set +e
  output=$(bash "$script_dir/check-vendor.sh" 2>&1)
  result=$?
  set -e
  assert_eq 1 "$result"
  assert_contains "$output" "$expected"
}

printf 'clean\n' >"$mode_file"
bash "$script_dir/check-vendor.sh" >/dev/null
run_case tidy 'go mod tidy changed the dependency graph'
run_case tracked 'M vendor/modules.txt'
run_case untracked '?? vendor/example/new.go'
assert_contains "$(<"$arguments")" '--untracked-files=all -- go.mod go.sum vendor'

printf 'PASS: tidy and vendor integrity\n'
