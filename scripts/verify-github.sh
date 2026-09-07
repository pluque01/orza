#!/usr/bin/env bash
set -euo pipefail

usage() {
  printf 'usage: %s --expect-visibility private|public\n' "${0##*/}" >&2
  exit 2
}

expected_visibility=
while [ "$#" -gt 0 ]; do
  case $1 in
    --expect-visibility)
      [ "$#" -ge 2 ] || usage
      expected_visibility=$2
      shift 2
      ;;
    *) usage ;;
  esac
done
case $expected_visibility in private|public) ;; *) usage ;; esac

repository=${ORZA_REPOSITORY:-pluque01/orza}
root=${ORZA_REPOSITORY_ROOT:-$(CDPATH='' cd -- "$(dirname -- "$0")/.." && pwd)}
gh_bin=${ORZA_GH_BIN:-gh}
curl_bin=${ORZA_CURL_BIN:-curl}
case $repository in
  [A-Za-z0-9_.-]*'/''/'*|*/*/*|/*|*/|*[!A-Za-z0-9_./-]*)
    printf 'invalid ORZA_REPOSITORY; expected owner/name\n' >&2
    exit 2
    ;;
esac

check_equal() {
  local label=$1 expected=$2 actual=$3
  if [ "$actual" != "$expected" ]; then
    printf 'verification failed: %s does not match the publication contract\n' "$label" >&2
    exit 1
  fi
}

[ -f "$root/README.md" ] || {
  printf 'verification failed: README.md is missing\n' >&2
  exit 1
}
grep -Fqx '# Orza' "$root/README.md" || {
  printf 'verification failed: product name Orza is missing\n' >&2
  exit 1
}
grep -Fqx 'Navigate remote systems from your terminal.' "$root/README.md" || {
  printf 'verification failed: product tagline is missing\n' >&2
  exit 1
}

check_equal 'repository name' 'orza' "$($gh_bin api "repos/$repository" --jq '.name')"
check_equal 'description' 'A terminal-native SSH client with a TUI' \
  "$($gh_bin api "repos/$repository" --jq '.description')"
check_equal 'visibility' "$expected_visibility" "$($gh_bin api "repos/$repository" --jq '.visibility')"
check_equal 'topics' 'cli,go,ssh,ssh-client,terminal,tui' \
  "$($gh_bin api "repos/$repository/topics" --jq '.names | sort | join(",")')"

for feature in secret_scanning secret_scanning_push_protection; do
  feature_status=$(
    "$gh_bin" api "repos/$repository" \
      --jq ".security_and_analysis.$feature.status // \"unavailable\""
  )
  case $feature_status in
    enabled) ;;
    unavailable)
      printf 'warning: %s is unavailable; record this platform limitation\n' "$feature" >&2
      ;;
    *)
      printf 'verification failed: %s is not enabled\n' "$feature" >&2
      exit 1
      ;;
  esac
done

if ! "$gh_bin" api "repos/$repository/private-vulnerability-reporting" >/dev/null 2>&1; then
  if [ "$expected_visibility" = private ]; then
    printf 'warning: private vulnerability reporting is unavailable before publication\n' >&2
  else
    printf 'verification failed: private vulnerability reporting is not enabled\n' >&2
    exit 1
  fi
fi
"$gh_bin" api "repos/$repository/vulnerability-alerts" >/dev/null
ruleset_id=$(
  "$gh_bin" api "repos/$repository/rulesets" \
    --jq '.[] | select(.name == "Protect main") | .id'
)
[ -n "$ruleset_id" ] || {
  printf 'verification failed: Protect main ruleset is missing\n' >&2
  exit 1
}
rules_ok=$(
  "$gh_bin" api "repos/$repository/rulesets/$ruleset_id" --jq \
    '(.name == "Protect main") and (.enforcement == "active") and (any(.rules[]; .type == "deletion")) and (any(.rules[]; .type == "non_fast_forward")) and (any(.rules[]; .type == "pull_request" and .parameters.required_approving_review_count == 0 and .parameters.required_review_thread_resolution == true)) and (any(.rules[]; .type == "required_status_checks" and any(.parameters.required_status_checks[]; .context == "CI / required")))'
)
check_equal 'main ruleset' true "$rules_ok"

status=$(env -u GH_TOKEN -u GITHUB_TOKEN "$curl_bin" --silent --show-error --output /dev/null \
  --write-out '%{http_code}' "https://api.github.com/repos/$repository")
if [ "$expected_visibility" = public ]; then
  check_equal 'unauthenticated repository visibility' 200 "$status"
elif [ "$status" = 200 ]; then
  printf 'verification failed: private repository is visible without authentication\n' >&2
  exit 1
fi

printf 'verified %s settings and %s unauthenticated visibility\n' "$repository" "$expected_visibility"
