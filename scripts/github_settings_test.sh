#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

gh_log=$TEST_TMPDIR/gh.log
curl_log=$TEST_TMPDIR/curl.log
export gh_log curl_log

stub_command gh <<'EOF'
printf '%s\n' '---' >>"$gh_log"
for argument in "$@"; do printf '<%s>\n' "$argument" >>"$gh_log"; done
if [[ " $* " == *" --input - "* ]]; then
  payload=$(cat)
  printf '<payload=%s>\n' "$payload" >>"$gh_log"
fi
query=${!#}
if [ "$query" = '.name' ]; then printf 'orza\n'; exit; fi
if [ "$query" = '.description' ]; then printf 'A terminal-native SSH client with a TUI\n'; exit; fi
if [ "$query" = '.visibility' ]; then printf '%s\n' "${mock_visibility:-private}"; exit; fi
if [ "$query" = '.names | sort | join(",")' ]; then printf 'cli,go,ssh,ssh-client,terminal,tui\n'; exit; fi
if [[ "$query" == *'.sha_pinning_required == true'* ]]; then printf 'true\n'; exit; fi
if [[ "$query" == *'cachix/install-nix-action@*'* ]]; then printf 'true\n'; exit; fi
if [[ "$query" == *'.default_workflow_permissions == "read"'* ]]; then printf 'true\n'; exit; fi
if [[ "$query" == '.security_and_analysis.'* ]]; then printf 'enabled\n'; exit; fi
if [[ "$query" == *'select(.name == "Protect main") | .id'* ]]; then printf '123\n'; exit; fi
if [[ "$query" == *'Protect main'* ]]; then printf 'true\n'; exit; fi
case "$*" in
  *'/private-vulnerability-reporting'*)
    [ "${mock_private_reporting:-enabled}" = enabled ] || exit 1
    printf 'enabled\n'
    ;;
  *'/vulnerability-alerts'*) printf 'enabled\n' ;;
  *'/rulesets'*)
    printf '%s\n' '[{"name":"Protect main","enforcement":"active","rules":[{"type":"deletion"},{"type":"non_fast_forward"},{"type":"pull_request","parameters":{"required_approving_review_count":0,"required_review_thread_resolution":true}},{"type":"required_status_checks","parameters":{"required_status_checks":[{"context":"required","integration_id":15368}]}}]}]'
    ;;
  *'/topics'*) printf '%s\n' '{"names":["cli","go","ssh","ssh-client","terminal","tui"]}' ;;
  *'repos/pluque01/orza'*) printf '%s\n' '{"name":"orza","description":"A terminal-native SSH client with a TUI","visibility":"private"}' ;;
esac
EOF

stub_command curl <<'EOF'
printf '%s\n' "$*" >>"$curl_log"
printf '%s' "${mock_http_status:-404}"
EOF

fixture=$TEST_TMPDIR/repository
mkdir -p "$fixture"
printf '# Orza\n\nNavigate remote systems from your terminal.\n' >"$fixture/README.md"

export ORZA_GH_BIN=$TEST_BIN/gh
export ORZA_CURL_BIN=$TEST_BIN/curl
export ORZA_REPOSITORY=pluque01/orza
export ORZA_REPOSITORY_ROOT=$fixture

"$script_dir/configure-github.sh"
configure_calls=$(<"$gh_log")
assert_contains "$configure_calls" '<description=A terminal-native SSH client with a TUI>'
for topic in cli go ssh ssh-client terminal tui; do
  assert_contains "$configure_calls" "<names[]=$topic>"
done
assert_contains "$configure_calls" '<repos/pluque01/orza/private-vulnerability-reporting>'
assert_contains "$configure_calls" '<repos/pluque01/orza/vulnerability-alerts>'
assert_contains "$configure_calls" '<repos/pluque01/orza/actions/permissions>'
assert_contains "$configure_calls" '"sha_pinning_required":true'
assert_contains "$configure_calls" '"patterns_allowed":["cachix/install-nix-action@*"]'
assert_contains "$configure_calls" '"default_workflow_permissions":"read"'
assert_contains "$configure_calls" '<security_and_analysis[secret_scanning][status]=enabled>'
assert_contains "$configure_calls" '<security_and_analysis[secret_scanning_push_protection][status]=enabled>'
assert_contains "$configure_calls" '<repos/pluque01/orza/rulesets>'
assert_contains "$configure_calls" '"required_approving_review_count":0'
assert_contains "$configure_calls" '"context":"required","integration_id":15368'
assert_contains "$configure_calls" '"type":"non_fast_forward"'
assert_contains "$configure_calls" '"type":"deletion"'
assert_not_contains "$configure_calls" '<visibility='

"$script_dir/verify-github.sh" --expect-visibility private
assert_contains "$(<"$curl_log")" 'https://api.github.com/repos/pluque01/orza'
export mock_visibility=public mock_http_status=200
"$script_dir/verify-github.sh" --expect-visibility public

export mock_visibility=private mock_http_status=404 mock_private_reporting=unavailable
output=$("$script_dir/configure-github.sh" 2>&1)
assert_contains "$output" 'private vulnerability reporting is unavailable'
output=$("$script_dir/verify-github.sh" --expect-visibility private 2>&1)
assert_contains "$output" 'private vulnerability reporting is unavailable before publication'
export mock_visibility=public mock_http_status=200
set +e
"$script_dir/verify-github.sh" --expect-visibility public >/dev/null 2>&1
unavailable_public_status=$?
set -e
assert_eq 1 "$unavailable_public_status"
unset mock_private_reporting

marker=$TEST_TMPDIR/hostile-side-effect
export ORZA_REPOSITORY="pluque01/orza;touch $marker"
set +e
"$script_dir/configure-github.sh" >/dev/null 2>&1
hostile_status=$?
set -e
assert_eq 2 "$hostile_status"
[ ! -e "$marker" ] || fail 'hostile repository value caused a side effect'

secret='SYNTHETIC_''GH_TOKEN_VALUE'
export GH_TOKEN=$secret
export ORZA_REPOSITORY=pluque01/orza
export mock_visibility=private mock_http_status=404
output=$("$script_dir/verify-github.sh" --expect-visibility private 2>&1)
assert_redacted "$secret" "$output"

printf 'PASS: mocked GitHub configuration and verification\n'
