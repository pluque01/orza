#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
root=$(CDPATH='' cd -- "$script_dir/.." && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"

readme=$(tr '\n' ' ' <"$root/README.md")

for asset in \
  orza_v1.2.3_linux_amd64.tar.gz \
  orza_v1.2.3_linux_arm64.tar.gz \
  orza_v1.2.3_darwin_amd64.tar.gz \
  orza_v1.2.3_darwin_arm64.tar.gz \
  orza_v1.2.3_windows_amd64.zip \
  orza_v1.2.3_windows_arm64.zip \
  orza_v1.2.3_checksums.txt; do
  assert_contains "$readme" "$asset"
done

# Literal documentation snippets intentionally contain shell and PowerShell variables.
# shellcheck disable=SC2016
for text in \
  'vMAJOR.MINOR.PATCH' \
  'git push origin v0.1.0' \
  'sha256sum --check --strict' \
  'shasum -a 256 --check' \
  'Get-FileHash -Algorithm SHA256' \
  '**do not authenticate the publisher or artifact by themselves**' \
  'install -m 0755 orza "$HOME/.local/bin/orza"' \
  'rm -- "$HOME/.local/bin/orza"' \
  'Copy-Item -Force .\orza-release\orza.exe' \
  'Remove-Item (Join-Path $env:LOCALAPPDATA "Programs\Orza\orza.exe")' \
  'Do not delete catalog directories' \
  'nix flake check --no-update-lock-file --keep-going -L' \
  'exactly Go 1.26.6' \
  'matching native Darwin builder with cgo enabled' \
  'cannot remember passwords in Keychain' \
  'proves source compatibility, not native terminal' \
  'VT-capable terminal' \
  '40x12' \
  'golang.org/x/crypto/ssh' \
  'Host identity is verified before' \
  'Passwords are never stored in SQLite' \
  'NO_COLOR=1 orza' \
  'shell.example.invalid' \
  'orza folder create /quickstart' \
  'orza connection create /quickstart/lab/shell' \
  'orza connection move /quickstart/lab/shell /quickstart' \
  'Press `q` from the browser or `Ctrl-C`' \
  '[contribution guidance](CONTRIBUTING.md)' \
  '[private security reporting](SECURITY.md)' \
  '[dependency vulnerability policy](SECURITY.md#dependency-findings)' \
  'actions/workflows/ci.yml/badge.svg' \
  'github.com/pluque01/orza/releases' \
  '[![License: MIT]'; do
  assert_contains "$readme" "$text"
done

assert_not_contains "$readme" '--host bastion.example.com'
assert_not_contains "$readme" '--host database.example.com'
assert_not_contains "$readme" '--host legacy.example.com'

printf 'PASS: README release, installation, usage, and safety contract\n'
