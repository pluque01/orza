#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

config=$script_dir/../.github/renovate.json
[ -f "$config" ] || fail 'Renovate configuration is missing'
jq -e . "$config" >/dev/null || fail 'Renovate configuration is not valid JSON'

assert_eq 5 "$(jq -r '.prConcurrentLimit' "$config")"
assert_eq 5 "$(jq -r '.branchConcurrentLimit' "$config")"
assert_eq false "$(jq -r '.automerge' "$config")"
assert_eq false "$(jq -r '.platformAutomerge' "$config")"
assert_eq false "$(jq -r '.vulnerabilityAlerts.enabled' "$config")"
assert_eq '3 days' "$(jq -r '.minimumReleaseAge' "$config")"
assert_eq true "$(jq -r '.dependencyDashboard' "$config")"

for manager in gomod github-actions nix; do
  jq -e --arg manager "$manager" '.enabledManagers | index($manager) != null' "$config" >/dev/null ||
    fail "missing manager: $manager"
done
jq -e '.packageRules[] | select(.matchManagers == ["gomod"] and (.postUpdateOptions | index("gomodTidy")))' "$config" >/dev/null ||
  fail 'Go updates must tidy and regenerate committed vendor source'
jq -e '(has("gomodSkipVendor") == false) and ([.packageRules[] | select(has("gomodSkipVendor"))] | length == 0)' "$config" >/dev/null ||
  fail 'obsolete gomodSkipVendor configuration must not be used'

for manager in gomod github-actions; do
  jq -e --arg manager "$manager" '.packageRules[] | select(.matchManagers == [$manager] and .groupName != null and (.matchUpdateTypes | index("minor")) and (.matchUpdateTypes | index("patch")))' "$config" >/dev/null ||
    fail "compatible updates are not grouped locally for $manager"
done
jq -e '.packageRules[] | select(.matchManagers == ["nix"] and .groupName != null and (.matchPackageNames | index("!nixpkgs")) and (.matchPackageNames | index("!vulndb")))' "$config" >/dev/null ||
  fail 'native Nix grouping must exclude nixpkgs and vulndb'
jq -e '.packageRules[] | select((.matchUpdateTypes | index("major")) and .dependencyDashboardApproval == true and .groupName == null)' "$config" >/dev/null ||
  fail 'major updates must be isolated behind dashboard approval'
jq -e '.packageRules[] | select(.matchManagers == ["github-actions"] and .rangeStrategy == "pin")' "$config" >/dev/null ||
  fail 'Actions must remain immutably pinned'
jq -e '[.packageRules[] | select(has("rangeStrategy") and has("matchUpdateTypes"))] | length == 0' "$config" >/dev/null ||
  fail 'rangeStrategy and matchUpdateTypes must be configured in separate rules'

jq -e '.packageRules[] | select(.matchManagers == ["nix"] and (.matchPackageNames | index("nixpkgs")) and .groupName == null)' "$config" >/dev/null ||
  fail 'nixpkgs must be handled separately'
jq -e '.packageRules[] | select(.matchManagers == ["custom.regex"] and (.matchPackageNames | index("golang/vulndb")) and .groupName == null and .postUpgradeTasks.commands == ["nix flake lock --update-input vulndb"] and (.postUpgradeTasks.fileFilters | index("flake.lock")))' "$config" >/dev/null ||
  fail 'vulndb must be separate and refresh flake.lock'
jq -e '.packageRules[] | select(.matchManagers == ["nix"] and (.matchPackageNames | index("vulndb")) and .enabled == false)' "$config" >/dev/null ||
  fail 'native Nix vulndb discovery must not duplicate the narrow fallback'
jq -e '.customManagers | length == 1 and .[0].managerFilePatterns == ["/^flake\\.nix$/"] and .[0].depNameTemplate == "golang/vulndb" and .[0].packageNameTemplate == "https://github.com/golang/vulndb" and .[0].datasourceTemplate == "git-refs" and .[0].currentValueTemplate == "master"' "$config" >/dev/null ||
  fail 'the vulndb fallback must be narrowly scoped to flake.nix'
jq -e '.customManagers[0].matchStrings[] | contains("(?<currentDigest>")' "$config" >/dev/null ||
  fail 'the vulndb fallback must capture the current digest'

notes=$(jq -r '.prBodyNotes[]' "$config")
assert_contains "$notes" 'Old and new versions'
assert_contains "$notes" 'affected'
assert_contains "$notes" 'ordinary CI'
assert_contains "$notes" 'explicit maintainer approval'

[ ! -e "$script_dir/../.github/dependabot.yml" ] || fail 'Dependabot update PRs would bypass the global Renovate ceiling'

printf 'PASS: Renovate Go/Actions/Nix policy contract\n'
