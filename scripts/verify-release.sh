#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'usage: %s --tag TAG --directory DIRECTORY [--write-manifest] [--revision SHA --release-json FILE]\n' "${0##*/}" >&2
  exit 2
}

tag=
directory=
revision=
release_json=
write_manifest=false
while [ "$#" -gt 0 ]; do
  case $1 in
    --tag) [ "$#" -ge 2 ] || usage; tag=$2; shift 2 ;;
    --directory) [ "$#" -ge 2 ] || usage; directory=$2; shift 2 ;;
    --revision) [ "$#" -ge 2 ] || usage; revision=$2; shift 2 ;;
    --release-json) [ "$#" -ge 2 ] || usage; release_json=$2; shift 2 ;;
    --write-manifest) write_manifest=true; shift ;;
    *) usage ;;
  esac
done

[[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || {
  printf 'verify-release: invalid stable tag: %q\n' "$tag" >&2
  exit 2
}
[ -d "$directory" ] || { printf 'verify-release: asset directory is missing\n' >&2; exit 1; }
directory=$(CDPATH='' cd -- "$directory" && pwd)

archives=(
  "orza_${tag}_darwin_amd64.tar.gz"
  "orza_${tag}_darwin_arm64.tar.gz"
  "orza_${tag}_linux_amd64.tar.gz"
  "orza_${tag}_linux_arm64.tar.gz"
  "orza_${tag}_windows_amd64.zip"
  "orza_${tag}_windows_arm64.zip"
)
manifest="orza_${tag}_checksums.txt"

if $write_manifest; then
  for asset in "${archives[@]}"; do
    [ -f "$directory/$asset" ] && [ ! -L "$directory/$asset" ] && [ -s "$directory/$asset" ] || {
      printf 'verify-release: missing or empty archive: %s\n' "$asset" >&2
      exit 1
    }
  done
  (cd -- "$directory" && LC_ALL=C sha256sum -- "${archives[@]}" >"$manifest")
fi

expected=$(printf '%s\n' "${archives[@]}" "$manifest" | LC_ALL=C sort)
actual=$(find "$directory" -mindepth 1 -maxdepth 1 -printf '%f\n' | LC_ALL=C sort)
[ "$actual" = "$expected" ] || {
  printf 'verify-release: exact seven-asset inventory mismatch\n' >&2
  exit 1
}

for asset in "${archives[@]}" "$manifest"; do
  [ -s "$directory/$asset" ] || { printf 'verify-release: zero-byte asset: %s\n' "$asset" >&2; exit 1; }
done

for asset in "${archives[@]}"; do
  case $asset in
    *.tar.gz)
      listing=$(tar -tzf "$directory/$asset")
      [ "$listing" = orza ] || { printf 'verify-release: %s must contain only root orza\n' "$asset" >&2; exit 1; }
      mode=$(tar -tvzf "$directory/$asset" | cut -c1-10)
      [[ $mode == -rwxr-xr-x ]] || { printf 'verify-release: %s executable mode is not 0755\n' "$asset" >&2; exit 1; }
      [ "$(tar -xOzf "$directory/$asset" -- orza | wc -c)" -gt 0 ] || { printf 'verify-release: %s contains an empty executable\n' "$asset" >&2; exit 1; }
      ;;
    *.zip)
      listing=$(unzip -Z1 "$directory/$asset")
      [ "$listing" = orza.exe ] || { printf 'verify-release: %s must contain only root orza.exe\n' "$asset" >&2; exit 1; }
      unzip -Z -v "$directory/$asset" | grep -F 'Unix file attributes (100755 octal)' >/dev/null || {
        printf 'verify-release: %s executable mode is not 0755\n' "$asset" >&2
        exit 1
      }
      [ "$(unzip -p "$directory/$asset" orza.exe | wc -c)" -gt 0 ] || { printf 'verify-release: %s contains an empty executable\n' "$asset" >&2; exit 1; }
      ;;
  esac
done

manifest_names=$(cut -d ' ' -f 3- "$directory/$manifest")
expected_names=$(printf '%s\n' "${archives[@]}" | LC_ALL=C sort)
[ "$manifest_names" = "$expected_names" ] || {
  printf 'verify-release: checksum manifest is not the exact filename-sorted archive inventory\n' >&2
  exit 1
}
(cd -- "$directory" && sha256sum --check --strict -- "$manifest" >/dev/null) || {
  printf 'verify-release: checksum verification failed\n' >&2
  exit 1
}

if [ -n "$release_json" ] || [ -n "$revision" ]; then
  [ -n "$release_json" ] && [ -f "$release_json" ] && [ -n "$revision" ] || usage
  jq -e --arg tag "$tag" --arg revision "$revision" --arg name "Orza $tag" \
    '.tagName == $tag and .targetCommitish == $revision and .name == $name and .isDraft == true and (.body | type == "string" and length > 0)' \
    "$release_json" >/dev/null || { printf 'verify-release: draft tag, target, name, state, or generated notes mismatch\n' >&2; exit 1; }
  json_assets=$(jq -r '.assets | sort_by(.name)[] | select(.size > 0) | .name' "$release_json")
  [ "$json_assets" = "$expected" ] || { printf 'verify-release: draft asset inventory mismatch\n' >&2; exit 1; }
fi

printf 'verify-release: exact release contract passed for %s\n' "$tag"
