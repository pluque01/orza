#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

tag=v1.2.3
revision=0123456789abcdef0123456789abcdef01234567
dist=$TEST_TMPDIR/'assets with spaces;inert'
mkdir -p "$dist"
for target in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64 windows-arm64; do
  binary=$TEST_TMPDIR/bin-$target
  printf 'synthetic %s version 1.2.3\n' "$target" >"$binary"
  bash "$script_dir/package-release.sh" --tag "$tag" --target "$target" --binary "$binary" --output "$dist" >/dev/null
done
bash "$script_dir/verify-release.sh" --tag "$tag" --directory "$dist" --write-manifest >/dev/null

release_json_fixture=$TEST_TMPDIR/release.json
jq -n --arg tag "$tag" --arg revision "$revision" --arg name "Orza $tag" \
  --arg body 'Generated changes for v1.2.3' \
  --argjson assets "$(for asset in "$dist"/*; do jq -n --arg name "${asset##*/}" --argjson size "$(stat -c %s "$asset")" '{name:$name,size:$size}'; done | jq -s .)" \
  '{tagName:$tag,targetCommitish:$revision,name:$name,isDraft:true,body:$body,assets:$assets}' >"$release_json_fixture"

gh_log=$TEST_TMPDIR/gh.log
public_marker=$TEST_TMPDIR/public
draft_marker=$TEST_TMPDIR/draft
export gh_log public_marker draft_marker release_json_fixture
stub_command gh <<'EOF'
printf '%s\n' '---' >>"$gh_log"
for argument in "$@"; do printf '<%s>\n' "$argument" >>"$gh_log"; done
case " $* " in
  *" release view "*" --json isDraft "*)
    case ${EXISTING_RELEASE:-none} in
      published) printf '%s\n' '{"isDraft":false}' ;;
      draft) printf '%s\n' '{"isDraft":true}' ;;
      none) exit 1 ;;
    esac
    ;;
  *" release delete "*)
    [ "${FAIL_STAGE:-}" != stale-draft-cleanup ] || exit 41
    rm -f -- "$draft_marker"
    ;;
  *" release create "*)
    [ "${FAIL_STAGE:-}" != draft-creation ] || exit 42
    : >"$draft_marker"
    ;;
  *" release upload "*)
    [ "${FAIL_STAGE:-}" != asset-upload ] || exit 43
    ;;
  *" release view "*" --json tagName,targetCommitish,name,isDraft,body,assets "*)
    [ "${FAIL_STAGE:-}" != draft-verification ] || printf '%s\n' '{"isDraft":true}'
    [ "${FAIL_STAGE:-}" != draft-verification ] || exit 0
    [ "${FAIL_STAGE:-}" != generated-notes ] || jq '.body = ""' "$release_json_fixture"
    [ "${FAIL_STAGE:-}" != generated-notes ] || exit 0
    cat "$release_json_fixture"
    ;;
  *" release edit "*)
    [ "${FAIL_STAGE:-}" != final-publication ] || exit 44
    : >"$public_marker"
    ;;
esac
EOF
export ORZA_GH_BIN=$TEST_BIN/gh

run_failure() {
  local failed_stage=$1
  rm -f -- "$public_marker" "$draft_marker"
  : >"$gh_log"
  export FAIL_STAGE=$failed_stage EXISTING_RELEASE=none
  set +e
  output=$(bash "$script_dir/publish-release.sh" --tag "$tag" --revision "$revision" --directory "$dist" 2>&1)
  status=$?
  set -e
  [ "$status" -ne 0 ] || fail "$failed_stage unexpectedly published"
  [ ! -e "$public_marker" ] || fail "$failed_stage left a stable public release"
  expected_stage=$failed_stage
  if [ "$failed_stage" = asset-upload ]; then expected_stage=upload; fi
  if [ "$failed_stage" = generated-notes ]; then expected_stage=draft-verification; fi
  assert_contains "$output" "stage=$expected_stage"
  assert_contains "$output" 'inspect this workflow step'
}

for failed_stage in draft-creation asset-upload draft-verification generated-notes final-publication; do
  run_failure "$failed_stage"
done

export FAIL_STAGE='' EXISTING_RELEASE=published
if bash "$script_dir/publish-release.sh" --tag "$tag" --revision "$revision" --directory "$dist" >/dev/null 2>&1; then
  fail 'duplicate published release was accepted'
fi
[ ! -e "$public_marker" ] || fail 'duplicate handling modified the public release'

rm -f -- "$public_marker" "$draft_marker"
: >"$gh_log"
export EXISTING_RELEASE=draft
bash "$script_dir/publish-release.sh" --tag "$tag" --revision "$revision" --directory "$dist" >/dev/null
assert_contains "$(<"$gh_log")" '<delete>'
assert_contains "$(<"$gh_log")" '<--cleanup-tag=false>'
[ -e "$public_marker" ] || fail 'verified stale draft replacement was not published'

rm -f -- "$public_marker"
: >"$gh_log"
export EXISTING_RELEASE=none
bash "$script_dir/publish-release.sh" --tag "$tag" --revision "$revision" --directory "$dist" >/dev/null
calls=$(<"$gh_log")
assert_contains "$calls" '<--draft>'
assert_contains "$calls" '<--generate-notes>'
assert_contains "$calls" '<--draft=false>'
[ -e "$public_marker" ] || fail 'successful exact draft was not published'

side_effect=$TEST_TMPDIR/hostile-side-effect
before=$(snapshot_tree "$dist")
if bash "$script_dir/publish-release.sh" --tag "v1.2.3;touch $side_effect" --revision "$revision" --directory "$dist" >/dev/null 2>&1; then
  fail 'hostile release tag was accepted'
fi
[ ! -e "$side_effect" ] || fail 'hostile release input executed a command'
assert_tree_unchanged "$before" "$dist"

printf 'PASS: draft-first mocked publication and failure paths\n'
