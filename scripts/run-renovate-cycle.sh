#!/usr/bin/env bash
set -euo pipefail

mode=${1:-weekly}
if [ "$#" -gt 1 ] || { [ "$mode" != weekly ] && [ "$mode" != manual ]; }; then
  printf 'usage: %s [weekly|manual]\n' "$0" >&2
  exit 2
fi

: "${RENOVATE_TOKEN:?run-renovate-cycle: RENOVATE_TOKEN is required}"
: "${RENOVATE_REPOSITORY:?run-renovate-cycle: RENOVATE_REPOSITORY is required}"
: "${RENOVATE_REVISION:?run-renovate-cycle: RENOVATE_REVISION is required}"

if [ "${GITHUB_ACTIONS:-}" = true ]; then
  printf '::add-mask::%s\n' "$RENOVATE_TOKEN"
fi
export GH_TOKEN=$RENOVATE_TOKEN

if [ "$mode" = weekly ]; then
  week_tag="automation/renovate/$(date -u +%G-W%V)"
  ref="refs/tags/$week_tag"
  marker_error=$(mktemp)
  trap 'rm -f "$marker_error"' EXIT HUP INT TERM
  if ! gh api --silent --method POST "repos/$RENOVATE_REPOSITORY/git/refs" \
    --field "ref=$ref" --field "sha=$RENOVATE_REVISION" 2>"$marker_error"; then
    if gh api --silent "repos/$RENOVATE_REPOSITORY/git/ref/tags/$week_tag" 2>/dev/null; then
      printf 'dependencies: UTC week marker %s already exists; no mutation performed\n' "$week_tag"
      exit 0
    fi
    printf 'dependencies: stage=week-marker failed; repository=%s; inspect this workflow step and rerun manual full dry-run\n' \
      "$RENOVATE_REPOSITORY" >&2
    exit 1
  fi
  printf 'dependencies: acquired UTC week marker %s; it will be retained on success or failure\n' "$week_tag"
else
  printf 'dependencies: manual cycle; weekly marker is not read or changed\n'
fi

if ! nix develop --command renovate --platform=github --require-config=required --config-validation-error=true "$RENOVATE_REPOSITORY"; then
  printf 'dependencies: stage=renovate failed; repository=%s; mode=%s; inspect this workflow step and run manual full dry-run\n' \
    "$RENOVATE_REPOSITORY" "$mode" >&2
  if [ "$mode" = weekly ]; then
    printf 'dependencies: marker=%s retained\n' "$week_tag" >&2
  fi
  exit 1
fi
