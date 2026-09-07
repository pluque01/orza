#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

scanner_log=$TEST_TMPDIR/scanner.log
export scanner_log
stub_command gitleaks <<'EOF'
printf '%s\n' "$*" >>"$scanner_log"
case " $* " in
  *" --redact "*) ;;
  *) printf 'scanner was not asked to redact output\n' >&2; exit 90 ;;
esac
case " $* " in
  *" git "*" --log-opts=--all "*) ;;
  *" git "*) printf 'history scan did not include every ref\n' >&2; exit 91 ;;
esac
target=${!#}
if [ -f "$target/seeded-secret" ]; then
  value=$(<"$target/seeded-secret")
  printf 'finding: %s\n' "$value" >&2
  exit 1
fi
EOF

export ORZA_GITLEAKS_BIN=$TEST_BIN/gitleaks
export ORZA_GITLEAKS_CONFIG=$script_dir/../.gitleaks.toml

clean=$TEST_TMPDIR/clean
mkdir -p "$clean"
printf 'public source\n' >"$clean/source.txt"
output=$("$script_dir/prepublish.sh" --repository "$clean" --history absent)
assert_contains "$output" 'sourceReview=passed'
assert_contains "$output" 'historyReview=not_present'

git_init_commit() {
  local repository=$1 message=$2
  git -C "$repository" init -q -b main
  git -C "$repository" config user.name 'Orza Test'
  git -C "$repository" config user.email 'orza-test@example.invalid'
  git -C "$repository" add .
  git -C "$repository" commit -q -m "$message"
}

new_history=$TEST_TMPDIR/new-history
mkdir -p "$new_history"
printf 'new source\n' >"$new_history/source.txt"
git_init_commit "$new_history" 'synthetic initial history'
output=$("$script_dir/prepublish.sh" --repository "$new_history" --history new)
assert_contains "$output" 'historyOrigin=new'
assert_contains "$output" 'historyReview=passed'

imported=$TEST_TMPDIR/imported
mkdir -p "$imported"
printf 'imported source\n' >"$imported/source.txt"
git_init_commit "$imported" 'synthetic imported history'
git -C "$imported" branch archived
git -C "$imported" tag v0.0.0-test
: >"$scanner_log"
output=$("$script_dir/prepublish.sh" --repository "$imported" --history imported)
assert_contains "$output" 'historyOrigin=imported'
assert_contains "$(<"$scanner_log")" '--log-opts=--all'

seeded=$TEST_TMPDIR/seeded
mkdir -p "$seeded"
synthetic_secret='SYNTHETIC_''SECRET_VALUE_FOR_GITLEAKS'
printf '%s\n' "$synthetic_secret" >"$seeded/seeded-secret"
set +e
failure_output=$("$script_dir/prepublish.sh" --repository "$seeded" --history absent 2>&1)
failure_status=$?
set -e
assert_eq 1 "$failure_status"
assert_contains "$failure_output" 'sourceReview=failed'
assert_redacted "$synthetic_secret" "$failure_output"

set +e
"$script_dir/prepublish.sh" --repository "$clean" --history imported >/dev/null 2>&1
invalid_transition=$?
set -e
assert_eq 2 "$invalid_transition"

printf 'PASS: prepublication tree and history policy\n'
