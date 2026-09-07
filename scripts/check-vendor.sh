#!/usr/bin/env bash
set -euo pipefail

git_bin=${ORZA_GIT_BIN:-git}
go_bin=${ORZA_GO_BIN:-go}

status() {
  "$git_bin" status --porcelain=v1 --untracked-files=all -- go.mod go.sum vendor
}

print_changes() {
  local changes=$1 line count=0
  while IFS= read -r line; do
    if [ "$count" -lt 100 ]; then
      printf '%s\n' "$line" >&2
    fi
    count=$((count + 1))
  done <<<"$changes"
  if [ "$count" -gt 100 ]; then
    printf '... %d additional changed paths omitted\n' "$((count - 100))" >&2
  fi
}

initial=$(status)
if [ -n "$initial" ]; then
  printf 'vendor integrity: checkout is not clean before regeneration\n' >&2
  print_changes "$initial"
  exit 1
fi

"$go_bin" mod tidy
tidy_changes=$(status)
if [ -n "$tidy_changes" ]; then
  printf 'vendor integrity: go mod tidy changed the dependency graph\n' >&2
  print_changes "$tidy_changes"
  exit 1
fi

"$go_bin" mod vendor
vendor_changes=$(status)
if [ -n "$vendor_changes" ]; then
  printf 'vendor integrity: go mod vendor changed tracked or untracked files\n' >&2
  print_changes "$vendor_changes"
  exit 1
fi

printf 'vendor integrity: tidy and vendor regeneration are clean\n'
