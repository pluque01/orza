#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
root=$(CDPATH='' cd -- "$script_dir/.." && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"

license=$(<"$root/LICENSE")
assert_contains "$license" 'MIT License'
assert_contains "$license" 'Copyright (c) 2026 Orza contributors'
assert_contains "$license" 'Permission is hereby granted, free of charge, to any person obtaining a copy'
assert_contains "$license" 'THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND'

security=$(<"$root/SECURITY.md")
assert_contains "$security" 'latest release'
assert_contains "$security" 'default branch'
assert_contains "$security" 'privately'
assert_contains "$security" 'private vulnerability reporting'
assert_contains "$security" 'coordinated disclosure'
assert_contains "$security" 'GO-YYYY-NNNN'
assert_contains "$security" 'reachability'
assert_not_contains "$security" '@gmail.com'

contributing=$(<"$root/CONTRIBUTING.md")
assert_contains "$contributing" 'nix develop'
assert_contains "$contributing" 'nix flake check --no-update-lock-file --keep-going -L'
assert_contains "$contributing" 'Pull requests'
assert_contains "$contributing" 'Sensitive Paths'
assert_contains "$contributing" 'vMAJOR.MINOR.PATCH'
assert_contains "$contributing" 'Never use a real secret'
assert_contains "$contributing" 'third-party license and notice files'

codeowners=$(<"$root/.github/CODEOWNERS")
assert_contains "$codeowners" '/.github/ @pluque01'
assert_contains "$codeowners" '/scripts/ @pluque01'
assert_not_contains "$codeowners" '/vendor/ @pluque01'
assert_contains "$codeowners" '/release.nix @pluque01'
assert_contains "$codeowners" '/SECURITY.md @pluque01'

assert_contains "$(<"$root/.gitignore")" 'vendor/'
assert_contains "$contributing" 'vendorHash'
printf 'PASS: repository publication documents and dependency ownership\n'
