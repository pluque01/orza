#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'usage: %s --repository PATH --history absent|new|imported\n' "${0##*/}" >&2
  exit 2
}

repository=.
history=
while [ "$#" -gt 0 ]; do
  case $1 in
    --repository)
      [ "$#" -ge 2 ] || usage
      repository=$2
      shift 2
      ;;
    --history)
      [ "$#" -ge 2 ] || usage
      history=$2
      shift 2
      ;;
    *) usage ;;
  esac
done

[ -d "$repository" ] || usage
case $history in
  absent)
    [ ! -d "$repository/.git" ] || {
      printf 'history transition invalid: absent history cannot contain .git\n' >&2
      exit 2
    }
    ;;
  new|imported)
    [ -d "$repository/.git" ] || {
      printf 'history transition invalid: %s history requires .git\n' "$history" >&2
      exit 2
    }
    ;;
  *) usage ;;
esac

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
gitleaks=${ORZA_GITLEAKS_BIN:-gitleaks}
config=${ORZA_GITLEAKS_CONFIG:-$script_dir/../.gitleaks.toml}
[ -f "$config" ] || {
  printf 'sourceReview=failed: gitleaks configuration is missing; rerun with ORZA_GITLEAKS_CONFIG\n' >&2
  exit 1
}

scan_log=$(mktemp "${TMPDIR:-/tmp}/orza-prepublish.XXXXXX")
trap 'rm -f "$scan_log"' EXIT HUP INT TERM

if ! "$gitleaks" dir --redact --config "$config" "$repository" >"$scan_log" 2>&1; then
  printf 'sourceReview=failed: redacted tree scan failed; reproduce with scripts/prepublish.sh\n' >&2
  exit 1
fi
printf 'sourceReview=passed\n'

if [ "$history" = absent ]; then
  printf 'historyReview=not_present\n'
  exit 0
fi

printf 'historyOrigin=%s\n' "$history"
if ! "$gitleaks" git --redact --config "$config" --log-opts=--all "$repository" >"$scan_log" 2>&1; then
  printf 'historyReview=failed: redacted all-ref history scan failed; reproduce with scripts/prepublish.sh\n' >&2
  exit 1
fi
printf 'historyReview=passed\n'
