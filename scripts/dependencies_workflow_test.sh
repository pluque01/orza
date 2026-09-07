#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

workflow_file=$script_dir/../.github/workflows/dependencies.yml
[ -f "$workflow_file" ] || fail 'dependency workflow is missing'
workflow=$(<"$workflow_file")

assert_eq 1 "$(printf '%s\n' "$workflow" | grep -c 'cron:')"
assert_contains "$workflow" "cron: '23 7 * * 1'"
assert_contains "$workflow" 'workflow_dispatch:'
assert_contains "$workflow" 'group: dependencies-weekly'
assert_contains "$workflow" 'cancel-in-progress: false'
# shellcheck disable=SC2016
assert_contains "$workflow" 'if: ${{ github.event_name == '\''schedule'\'' }}'
# shellcheck disable=SC2016
assert_contains "$workflow" 'if: ${{ github.event_name == '\''workflow_dispatch'\'' }}'
# shellcheck disable=SC2016
assert_contains "$workflow" 'RENOVATE_TOKEN: ${{ secrets.RENOVATE_TOKEN }}'
assert_eq 1 "$(printf '%s\n' "$workflow" | grep -c 'secrets.RENOVATE_TOKEN')"
assert_contains "$workflow" 'RENOVATE_ALLOWED_COMMANDS:'
assert_contains "$workflow" '^nix flake lock --update-input vulndb$'
assert_contains "$workflow" '--platform=local --dry-run=full --require-config=required'
assert_not_contains "$workflow" 'cancel-in-progress: true'
assert_not_contains "$workflow" 'pull_request_target:'
assert_not_contains "$workflow" 'contents: write'
assert_not_contains "$workflow" 'env:'$'\n''  RENOVATE_TOKEN:'

checkout_count=0
while IFS= read -r line; do
  case $line in
    *'uses: actions/checkout@'*) checkout_count=$((checkout_count + 1)) ;;
    *'persist-credentials: false'*) checkout_count=$((checkout_count - 1)) ;;
    *'uses: '*)
      reference=${line#*@}
      sha=${reference%% *}
      comment=${reference#*# }
      [[ $sha =~ ^[0-9a-f]{40}$ ]] || fail "action is not pinned to a full SHA: $line"
      [ "$comment" != "$reference" ] && [ -n "$comment" ] || fail "action pin lacks a version comment: $line"
      ;;
  esac
done <<<"$workflow"
assert_eq 0 "$checkout_count"

cycle=$(<"$script_dir/run-renovate-cycle.sh")
assert_contains "$cycle" '::add-mask::'
assert_contains "$cycle" 'date -u +%G-W%V'
assert_contains "$cycle" 'marker='
assert_not_contains "$cycle" 'set -x'
assert_not_contains "$workflow" 'printenv'

printf 'PASS: scheduled/manual dependency workflow security contract\n'
