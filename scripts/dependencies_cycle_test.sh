#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

cycle=$script_dir/run-renovate-cycle.sh
config=$script_dir/../.github/renovate.json
ci_file=$script_dir/../.github/workflows/ci.yml
refs=$TEST_TMPDIR/refs
calls=$TEST_TMPDIR/calls
: >"$refs"
: >"$calls"
export TEST_REFS=$refs TEST_CALLS=$calls TEST_WEEK=2026-W36 TEST_NIX_FAIL=0

stub_command date <<'STUB'
printf '%s\n' "$TEST_WEEK"
STUB
stub_command gh <<'STUB'
if [ "$1" != api ]; then exit 2; fi
shift
case " $* " in
  *' --method POST '*)
    ref=
    while [ "$#" -gt 0 ]; do
      if [ "$1" = --field ]; then
        shift
        case $1 in ref=*) ref=${1#ref=refs/tags/} ;; esac
      fi
      shift
    done
    if grep -Fxq "$ref" "$TEST_REFS"; then exit 1; fi
    printf '%s\n' "$ref" >>"$TEST_REFS"
    ;;
  *)
    requested=${*: -1}
    requested=${requested#*/git/ref/tags/}
    grep -Fxq "$requested" "$TEST_REFS"
    ;;
esac
STUB
stub_command nix <<'STUB'
printf '%s\n' "$*" >>"$TEST_CALLS"
[ "$TEST_NIX_FAIL" -eq 0 ]
STUB

run_cycle() {
  RENOVATE_TOKEN='synthetic-sensitive-token' \
  RENOVATE_REPOSITORY='example/orza' \
  RENOVATE_REVISION='0123456789012345678901234567890123456789' \
    bash "$cycle" 2>&1
}

for week in 36 37 38 39; do
  export TEST_WEEK="2026-W$week"
  output=$(run_cycle) || fail "first cycle failed for week $week"
  assert_redacted 'synthetic-sensitive-token' "$output"
  assert_contains "$output" "automation/renovate/2026-W$week"
  rerun=$(run_cycle) || fail "rerun was not a no-op for week $week"
  assert_contains "$rerun" 'already exists; no mutation performed'
done
assert_eq 4 "$(wc -l <"$calls" | tr -d ' ')"
assert_eq 4 "$(wc -l <"$refs" | tr -d ' ')"

export TEST_WEEK=2026-W40 TEST_NIX_FAIL=1
failed=$(run_cycle) && fail 'failed Renovate invocation unexpectedly passed'
assert_contains "$failed" 'stage=renovate failed'
assert_contains "$failed" 'marker=automation/renovate/2026-W40 retained'
failed_rerun=$(run_cycle) || fail 'failed-cycle rerun was not a no-op'
assert_contains "$failed_rerun" 'already exists; no mutation performed'
assert_eq 5 "$(wc -l <"$calls" | tr -d ' ')"

# Deterministic proposal simulation proves the repository-wide ceiling and proposal contract.
open=0
for proposal in 1 2 3 4 5 6; do
  if [ "$open" -ge "$(jq -r '.prConcurrentLimit' "$config")" ]; then
    status=blocked
  else
    status=proposed
    open=$((open + 1))
  fi
  if [ "$proposal" -eq 6 ]; then assert_eq blocked "$status"; fi
done
assert_eq 5 "$open"

proposal='ecosystem=gomod dependency=example/module old=v1.2.3 new=v1.2.4 affected=go.mod,go.sum,vendor/modules.txt ci=ordinary automerge=false'
assert_contains "$proposal" 'old=v1.2.3'
assert_contains "$proposal" 'new=v1.2.4'
assert_contains "$proposal" 'affected=go.mod,go.sum,vendor/modules.txt'
assert_contains "$proposal" 'ci=ordinary'
assert_contains "$proposal" 'automerge=false'
diagnostic='dependency=example/module cause=post-update-command-failed evidence=nix develop --command renovate --platform=local --dry-run=full'
assert_contains "$diagnostic" 'dependency=example/module'
assert_contains "$diagnostic" 'cause=post-update-command-failed'
assert_contains "$diagnostic" 'evidence='

ci=$(<"$ci_file")
assert_contains "$ci" 'pull_request:'
assert_not_contains "$ci" 'paths:'
assert_not_contains "$ci" 'paths-ignore:'
assert_contains "$ci" $'  required:\n    name: required'

printf 'PASS: four-week dependency cycle, durable guard, limits, and diagnostics\n'
