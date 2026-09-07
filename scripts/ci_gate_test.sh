#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

categories=(format test race vet static vulnerability build vendor secret)
passing=()
for category in "${categories[@]}"; do passing+=("$category=success"); done

bash "$script_dir/ci-gate.sh" "${passing[@]}" >/dev/null

for failed_category in "${categories[@]}"; do
  simulation=()
  for category in "${categories[@]}"; do
    status=success
    [ "$category" != "$failed_category" ] || status=failure
    simulation+=("$category=$status")
  done
  set +e
  output=$(bash "$script_dir/ci-gate.sh" "${simulation[@]}" 2>&1)
  result=$?
  set -e
  assert_eq 1 "$result"
  assert_contains "$output" "$failed_category did not pass"
done

excepted=("${passing[@]}")
excepted[5]='vulnerability=excepted'
bash "$script_dir/ci-gate.sh" "${excepted[@]}" >/dev/null

set +e
output=$(bash "$script_dir/ci-gate.sh" "${passing[@]:1}" 2>&1)
result=$?
set -e
assert_eq 1 "$result"
assert_contains "$output" 'required check format is missing'

printf 'PASS: stable aggregate gate simulations\n'
