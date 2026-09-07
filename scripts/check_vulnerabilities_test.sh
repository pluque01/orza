#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
. "$script_dir/testlib.sh"
testlib_init

arguments_log=$TEST_TMPDIR/scanner-arguments
policy_log=$TEST_TMPDIR/policy-arguments
export arguments_log policy_log

stub_command govulncheck-stub <<'EOF'
: >"$arguments_log"
for argument in "$@"; do
  printf '<%s>\n' "$argument" >>"$arguments_log"
done
printf '%s\n' '{"config":{"protocol_version":"v1.0.0"}}'
EOF

stub_command policy-stub <<'EOF'
: >"$policy_log"
for argument in "$@"; do
  printf '<%s>\n' "$argument" >>"$policy_log"
done
while IFS= read -r _; do :; done
EOF

export ORZA_GOVULNCHECK_BIN=$TEST_BIN/govulncheck-stub
export ORZA_POLICY_BIN=$TEST_BIN/policy-stub
export ORZA_VULNDB=$TEST_TMPDIR/database
export ORZA_VULNERABILITY_EXCEPTIONS=$TEST_TMPDIR/exceptions.json
printf '[]\n' >"$ORZA_VULNERABILITY_EXCEPTIONS"
mkdir -p "$ORZA_VULNDB"

"$script_dir/check-vulnerabilities.sh" pinned './package with spaces/...'
arguments=$(<"$arguments_log")
assert_contains "$arguments" '<-json>'
assert_contains "$arguments" '<-db>'
assert_contains "$arguments" "<file://$ORZA_VULNDB>"
assert_contains "$arguments" '<./package with spaces/...>'

"$script_dir/check-vulnerabilities.sh" live ./internal/...
arguments=$(<"$arguments_log")
assert_not_contains "$arguments" '<-db>'
assert_contains "$arguments" '<./internal/...>'

stub_command scanner-findings <<'EOF'
printf '%s\n' '{"config":{"protocol_version":"v1.0.0"}}' '{"finding":{"osv":"GO-2026-5972","trace":[{"function":"synthetic"}]}}'
exit 3
EOF
export ORZA_GOVULNCHECK_BIN=$TEST_BIN/scanner-findings
"$script_dir/check-vulnerabilities.sh" live

stub_command scanner-failure <<'EOF'
printf '%s\n' 'SYNTHETIC_SECRET_VALUE' >&2
exit 23
EOF
export ORZA_GOVULNCHECK_BIN=$TEST_BIN/scanner-failure
set +e
failure_output=$("$script_dir/check-vulnerabilities.sh" live 2>&1)
failure_status=$?
set -e
assert_eq 23 "$failure_status"
assert_contains "$failure_output" 'exit status 23'
assert_redacted 'SYNTHETIC_SECRET_VALUE' "$failure_output"

side_effect_root=$TEST_TMPDIR/no-side-effects
mkdir -p "$side_effect_root"
printf 'sentinel\n' >"$side_effect_root/sentinel"
before=$(snapshot_tree "$side_effect_root")
set +e
(cd "$side_effect_root" && "$script_dir/check-vulnerabilities.sh" invalid >/dev/null 2>&1)
invalid_status=$?
set -e
assert_eq 2 "$invalid_status"
assert_tree_unchanged "$before" "$side_effect_root"

printf 'PASS: vulnerability wrapper\n'
