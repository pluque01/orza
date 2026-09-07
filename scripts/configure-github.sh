#!/usr/bin/env bash
set -euo pipefail

repository=${ORZA_REPOSITORY:-pluque01/orza}
gh_bin=${ORZA_GH_BIN:-gh}

case $repository in
  [A-Za-z0-9_.-]*'/''/'*|*/*/*|/*|*/|*[!A-Za-z0-9_./-]*)
    printf 'invalid ORZA_REPOSITORY; expected owner/name\n' >&2
    exit 2
    ;;
esac
owner=${repository%%/*}
name=${repository#*/}
[ -n "$owner" ] && [ -n "$name" ] || exit 2

description='A terminal-native SSH client with a TUI'
ruleset='{"name":"Protect main","target":"branch","enforcement":"active","conditions":{"ref_name":{"include":["~DEFAULT_BRANCH"],"exclude":[]}},"rules":[{"type":"deletion"},{"type":"non_fast_forward"},{"type":"pull_request","parameters":{"dismiss_stale_reviews_on_push":false,"require_code_owner_review":false,"require_last_push_approval":false,"required_approving_review_count":0,"required_review_thread_resolution":true}},{"type":"required_status_checks","parameters":{"strict_required_status_checks_policy":true,"do_not_enforce_on_create":false,"required_status_checks":[{"context":"CI / required"}]}}],"bypass_actors":[]}'

"$gh_bin" api --method PATCH "repos/$repository" -f "description=$description" >/dev/null
"$gh_bin" api --method PUT "repos/$repository/topics" \
  -f 'names[]=cli' -f 'names[]=go' -f 'names[]=ssh' -f 'names[]=ssh-client' \
  -f 'names[]=terminal' -f 'names[]=tui' >/dev/null
if ! "$gh_bin" api --method PUT "repos/$repository/private-vulnerability-reporting" >/dev/null 2>&1; then
  printf 'warning: private vulnerability reporting is unavailable; enable it after publication\n' >&2
fi
"$gh_bin" api --method PUT "repos/$repository/vulnerability-alerts" >/dev/null

# Private repositories without the relevant GitHub plan may reject these controls.
if ! "$gh_bin" api --method PATCH "repos/$repository" \
  -F 'security_and_analysis[secret_scanning][status]=enabled' \
  -F 'security_and_analysis[secret_scanning_push_protection][status]=enabled' >/dev/null 2>&1; then
  printf 'warning: secret scanning or push protection is unavailable; record this platform limitation\n' >&2
fi

existing_ruleset=$(
  "$gh_bin" api "repos/$repository/rulesets" \
    --jq '.[] | select(.name == "Protect main") | .id' 2>/dev/null || true
)
if [ -n "$existing_ruleset" ]; then
  "$gh_bin" api --method PUT "repos/$repository/rulesets/$existing_ruleset" --input - >/dev/null <<<"$ruleset"
else
  "$gh_bin" api --method POST "repos/$repository/rulesets" --input - >/dev/null <<<"$ruleset"
fi

printf 'configured %s without changing repository visibility\n' "$repository"
