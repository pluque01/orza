#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
repository_root=$(CDPATH='' cd -- "$script_dir/.." && pwd)

if [ "$#" -lt 1 ]; then
  printf 'usage: %s pinned|live [govulncheck package ...]\n' "$0" >&2
  exit 2
fi

mode=$1
shift
case $mode in
  pinned | live) ;;
  *)
    printf 'vulnerability scan: mode must be pinned or live\n' >&2
    exit 2
    ;;
esac

if [ "$#" -eq 0 ]; then
  set -- ./...
fi

scanner=${ORZA_GOVULNCHECK_BIN:-govulncheck}
exceptions=${ORZA_VULNERABILITY_EXCEPTIONS:-$repository_root/.vulnerability-exceptions.json}
temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/orza-vulnerability-scan.XXXXXX")
trap 'rm -rf "$temporary_directory"' EXIT HUP INT TERM
stream=$temporary_directory/govulncheck.json
scanner_log=$temporary_directory/govulncheck.stderr

scanner_arguments=(-json)
if [ "$mode" = pinned ]; then
  if [ -z "${ORZA_VULNDB:-}" ]; then
    printf 'vulnerability scan: ORZA_VULNDB is required in pinned mode; enter the Nix development shell\n' >&2
    exit 2
  fi
  scanner_arguments+=(-db "file://$ORZA_VULNDB")
fi
scanner_arguments+=("$@")

set +e
"$scanner" "${scanner_arguments[@]}" >"$stream" 2>"$scanner_log"
scanner_status=$?
set -e
if [ "$scanner_status" -ne 0 ] && [ "$scanner_status" -ne 3 ]; then
  printf 'vulnerability scan failed with exit status %d; rerun scripts/check-vulnerabilities.sh %s\n' "$scanner_status" "$mode" >&2
  exit "$scanner_status"
fi

policy_arguments=(-exceptions "$exceptions")
if [ -n "${ORZA_POLICY_DATE:-}" ]; then
  policy_arguments+=(-date "$ORZA_POLICY_DATE")
fi
if [ -n "${ORZA_POLICY_BIN:-}" ]; then
  "$ORZA_POLICY_BIN" "${policy_arguments[@]}" <"$stream"
else
  (
    cd "$repository_root"
    go run ./cmd/vulncheck-policy "${policy_arguments[@]}" <"$stream"
  )
fi
