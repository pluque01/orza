#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'usage: %s --tag TAG --revision SHA --directory DIRECTORY\n' "${0##*/}" >&2
  exit 2
}

tag=
revision=
directory=
while [ "$#" -gt 0 ]; do
  case $1 in
    --tag) [ "$#" -ge 2 ] || usage; tag=$2; shift 2 ;;
    --revision) [ "$#" -ge 2 ] || usage; revision=$2; shift 2 ;;
    --directory) [ "$#" -ge 2 ] || usage; directory=$2; shift 2 ;;
    *) usage ;;
  esac
done

[[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || { printf 'publish-release: eligibility: invalid stable tag\n' >&2; exit 2; }
[[ $revision =~ ^[0-9a-f]{40}$ ]] || { printf 'publish-release: eligibility: invalid revision\n' >&2; exit 2; }
[ -d "$directory" ] || { printf 'publish-release: assembly: asset directory is missing\n' >&2; exit 1; }
directory=$(CDPATH='' cd -- "$directory" && pwd)
gh_bin=${ORZA_GH_BIN:-gh}
verify_bin=${ORZA_VERIFY_RELEASE_BIN:-$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)/verify-release.sh}
stage=eligibility
on_exit() {
  local status=$1
  if [ "$status" -ne 0 ]; then
    printf 'publish-release: stage=%s failed; inspect this workflow step and rerun scripts/release_publish_test.sh\n' "$stage" >&2
  fi
  exit "$status"
}
trap 'on_exit "$?"' EXIT

stage=assembly
bash "$verify_bin" --tag "$tag" --directory "$directory" >/dev/null

existing=$("$gh_bin" release view "$tag" --json isDraft 2>/dev/null || true)
if [ -n "$existing" ]; then
  if [ "$(printf '%s' "$existing" | jq -r '.isDraft')" != true ]; then
    printf 'publish-release: eligibility: published release already exists for %s\n' "$tag" >&2
    exit 1
  fi
  stage=stale-draft-cleanup
  "$gh_bin" release delete "$tag" --yes --cleanup-tag=false
fi

stage=draft-creation
"$gh_bin" release create "$tag" --draft --generate-notes --title "Orza $tag" --target "$revision"

assets=()
while IFS= read -r asset; do assets+=("$directory/$asset"); done < <(find "$directory" -mindepth 1 -maxdepth 1 -type f -printf '%f\n' | LC_ALL=C sort)
stage=upload
"$gh_bin" release upload "$tag" "${assets[@]}"

stage=draft-verification
release_json=$(mktemp "${TMPDIR:-/tmp}/orza-release.XXXXXX.json")
"$gh_bin" release view "$tag" --json tagName,targetCommitish,name,isDraft,body,assets >"$release_json"
bash "$verify_bin" --tag "$tag" --directory "$directory" --revision "$revision" --release-json "$release_json"
rm -f -- "$release_json"

stage=final-publication
"$gh_bin" release edit "$tag" --draft=false
stage=published
printf 'publish-release: published %s after exact draft verification\n' "$tag"
