#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

workflow_file=$script_dir/../.github/workflows/release.yml
[ -f "$workflow_file" ] || fail 'release workflow is missing'
workflow=$(<"$workflow_file")

valid_tag() { [[ $1 =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; }
for tag in v0.0.0 v1.2.3 v10.20.30; do valid_tag "$tag" || fail "valid tag rejected: $tag"; done
for tag in 1.2.3 v1.2 v1.2.3.4 v01.2.3 v1.02.3 v1.2.03 'v1.2.3;touch x'; do
  if valid_tag "$tag"; then fail "invalid tag accepted: $tag"; fi
done

assert_contains "$workflow" 'tags:'
assert_contains "$workflow" "- 'v*'"
assert_contains "$workflow" 'git fetch --no-tags origin main'
# shellcheck disable=SC2016
assert_contains "$workflow" 'git merge-base --is-ancestor "$RELEASE_REVISION" refs/remotes/origin/main'
assert_contains "$workflow" 'check-runs'
assert_contains "$workflow" '.name == "required"'
assert_contains "$workflow" '.status == "completed"'
assert_contains "$workflow" '.conclusion == "success"'
assert_contains "$workflow" 'required CI check is absent or failed'
assert_contains "$workflow" 'published release already exists'
assert_contains "$workflow" 'stale draft will be replaced'
# shellcheck disable=SC2016
assert_contains "$workflow" 'group: release-${{ github.ref_name }}'
assert_contains "$workflow" 'cancel-in-progress: false'
assert_contains "$workflow" 'GO_VERSION'
# shellcheck disable=SC2016
assert_contains "$workflow" 'go-version: ${{ needs.eligibility.outputs.go-version }}'
assert_contains "$(<"$script_dir/../.github/release.env")" 'GO_VERSION=1.26.6'

assert_not_contains "$workflow" 'pull_request_target:'
assert_contains "$workflow" $'permissions:\n  contents: read'
assert_eq 1 "$(printf '%s\n' "$workflow" | grep -c 'contents: write')"

printf 'PASS: release eligibility and same-tag concurrency contract\n'
