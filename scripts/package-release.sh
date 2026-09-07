#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'usage: %s --tag vMAJOR.MINOR.PATCH --target OS-ARCH --binary PATH --output DIRECTORY\n' "${0##*/}" >&2
  exit 2
}

tag=
target=
binary=
output=
while [ "$#" -gt 0 ]; do
  case $1 in
    --tag) [ "$#" -ge 2 ] || usage; tag=$2; shift 2 ;;
    --target) [ "$#" -ge 2 ] || usage; target=$2; shift 2 ;;
    --binary) [ "$#" -ge 2 ] || usage; binary=$2; shift 2 ;;
    --output) [ "$#" -ge 2 ] || usage; output=$2; shift 2 ;;
    *) usage ;;
  esac
done

[[ $tag =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]] || {
  printf 'package-release: invalid stable tag: %q\n' "$tag" >&2
  exit 2
}

case $target in
  linux-amd64|linux-arm64|darwin-amd64|darwin-arm64)
    executable=orza
    extension=tar.gz
    ;;
  windows-amd64|windows-arm64)
    executable=orza.exe
    extension=zip
    ;;
  *)
    printf 'package-release: unsupported target: %q\n' "$target" >&2
    exit 2
    ;;
esac

[ -f "$binary" ] && [ ! -L "$binary" ] && [ -s "$binary" ] || {
  printf 'package-release: target %s binary must be a non-empty regular file\n' "$target" >&2
  exit 1
}
[ -n "$output" ] || usage
mkdir -p -- "$output"
output=$(CDPATH='' cd -- "$output" && pwd)

stage=$(mktemp -d "${TMPDIR:-/tmp}/orza-package.XXXXXX")
trap 'rm -rf "$stage"' EXIT HUP INT TERM
install -m 0755 -- "$binary" "$stage/$executable"
archive="orza_${tag}_${target%-*}_${target##*-}.${extension}"

case $extension in
  tar.gz)
    tar --sort=name --mtime=@0 --owner=0 --group=0 --numeric-owner --format=ustar \
      -C "$stage" -czf "$output/$archive" -- "$executable"
    ;;
  zip)
    TZ=UTC touch -t 198001010000 -- "$stage/$executable"
    (cd -- "$stage" && TZ=UTC zip -X -q "$output/$archive" "$executable")
    ;;
esac

[ -s "$output/$archive" ] || {
  printf 'package-release: target %s produced an empty archive\n' "$target" >&2
  exit 1
}
printf '%s\n' "$archive"
