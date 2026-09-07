#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

ci=$script_dir/../.github/workflows/ci.yml
monitor=$script_dir/../.github/workflows/vulnerability-monitor.yml
[ -f "$ci" ] || fail 'CI workflow is missing'
[ -f "$monitor" ] || fail 'vulnerability monitor workflow is missing'
ci_text=$(<"$ci")
monitor_text=$(<"$monitor")

assert_contains "$ci_text" 'name: CI'
for event in pull_request push merge_group workflow_dispatch; do
  assert_contains "$ci_text" "  $event:"
done
assert_contains "$ci_text" 'branches: [main]'
assert_not_contains "$ci_text" 'paths:'
assert_not_contains "$ci_text" 'paths-ignore:'
assert_not_contains "$ci_text" 'pull_request_target:'
assert_contains "$ci_text" 'permissions:'
assert_contains "$ci_text" 'contents: read'
assert_not_contains "$ci_text" 'contents: write'
assert_contains "$ci_text" "cancel-in-progress: \${{ github.event_name == 'pull_request' }}"

for job in quality vendor release-builds secrets required; do
  assert_contains "$ci_text" "  $job:"
done
assert_contains "$ci_text" "if: \${{ always() }}"
assert_contains "$ci_text" 'nix flake check --no-update-lock-file --keep-going -L'
assert_contains "$ci_text" 'scripts/check-vendor.sh'
assert_contains "$ci_text" '--untracked-files=all'
assert_contains "$ci_text" 'linux-amd64'
assert_contains "$ci_text" 'linux-arm64'
assert_contains "$ci_text" 'windows-amd64'
assert_contains "$ci_text" 'windows-arm64'
assert_contains "$ci_text" 'gitleaks dir --redact'
assert_contains "$ci_text" 'gitleaks git --redact'

combined=$(printf '%s\n%s\n' "$ci_text" "$monitor_text")
assert_not_contains "$combined" 'self-hosted'
assert_not_contains "$combined" ': write'
assert_contains "$ci_text" $'  required:\n    name: required'
assert_contains "$ci_text" 'needs: [quality, vendor, release-builds, secrets]'
while IFS= read -r line; do
  case $line in
    *'uses: '*)
      reference=${line#*@}
      sha=${reference%% *}
      comment=${reference#*# }
      [[ $sha =~ ^[0-9a-f]{40}$ ]] || fail "action is not pinned to a full SHA: $line"
      [ "$comment" != "$reference" ] && [ -n "$comment" ] || fail "action pin lacks a version comment: $line"
      ;;
  esac
done <<<"$combined"

checkout_count=0
while IFS= read -r line; do
  case $line in
    *'uses: actions/checkout@'*) checkout_count=$((checkout_count + 1)) ;;
    *'persist-credentials: false'*) checkout_count=$((checkout_count - 1)) ;;
  esac
done <<<"$combined"
assert_eq 0 "$checkout_count"

runner_count=$(printf '%s\n' "$combined" | { grep -c 'runs-on: ubuntu-24.04' || true; })
[ "$runner_count" -ge 5 ] || fail 'every job must use an explicit hosted runner'
assert_contains "$monitor_text" 'schedule:'
assert_contains "$monitor_text" 'workflow_dispatch:'
assert_contains "$monitor_text" 'scripts/check-vulnerabilities.sh live'
assert_contains "$monitor_text" 'cancel-in-progress: false'

printf 'PASS: CI and vulnerability workflow contract\n'
