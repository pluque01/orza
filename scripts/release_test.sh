#!/usr/bin/env bash
set -euo pipefail

script_dir=$(CDPATH='' cd -- "$(dirname -- "$0")" && pwd)
# shellcheck source=scripts/testlib.sh
# shellcheck disable=SC1091
. "$script_dir/testlib.sh"
testlib_init

tag=v1.2.3
targets=(linux-amd64 linux-arm64 darwin-amd64 darwin-arm64 windows-amd64 windows-arm64)
dist=$TEST_TMPDIR/dist
repeat=$TEST_TMPDIR/repeat
mkdir -p "$dist" "$repeat"

for target in "${targets[@]}"; do
  binary=$TEST_TMPDIR/orza-$target
  printf 'synthetic executable target=%s version=1.2.3\n' "$target" >"$binary"
  bash "$script_dir/package-release.sh" --tag "$tag" --target "$target" --binary "$binary" --output "$dist" >/dev/null
  bash "$script_dir/package-release.sh" --tag "$tag" --target "$target" --binary "$binary" --output "$repeat" >/dev/null
done

bash "$script_dir/verify-release.sh" --tag "$tag" --directory "$dist" --write-manifest >/dev/null
bash "$script_dir/verify-release.sh" --tag "$tag" --directory "$repeat" --write-manifest >/dev/null
for asset in "$dist"/*; do
  name=${asset##*/}
  assert_eq "$(sha256sum "$asset" | cut -d ' ' -f 1)" "$(sha256sum "$repeat/$name" | cut -d ' ' -f 1)"
done

assert_eq 7 "$(find "$dist" -mindepth 1 -maxdepth 1 -type f | wc -l)"
manifest_names=$(cut -d ' ' -f 3- "$dist/orza_${tag}_checksums.txt")
assert_eq "$(printf '%s\n' "$manifest_names" | LC_ALL=C sort)" "$manifest_names"
assert_contains "$(tar -tvzf "$dist/orza_${tag}_linux_amd64.tar.gz")" '-rwxr-xr-x'
assert_eq orza.exe "$(unzip -Z1 "$dist/orza_${tag}_windows_amd64.zip")"
assert_contains "$(unzip -Z -v "$dist/orza_${tag}_windows_amd64.zip")" 'Unix file attributes (100755 octal)'

expect_verify_failure() {
  local fixture=$1
  shift
  cp -a "$dist" "$fixture"
  "$@"
  if bash "$script_dir/verify-release.sh" --tag "$tag" --directory "$fixture" >/dev/null 2>&1; then
    fail "release verification unexpectedly accepted ${fixture##*/}"
  fi
}

missing=$TEST_TMPDIR/missing
expect_verify_failure "$missing" rm "$missing/orza_${tag}_linux_arm64.tar.gz"
extra=$TEST_TMPDIR/extra
expect_verify_failure "$extra" touch "$extra/unexpected.asset"
empty=$TEST_TMPDIR/empty
expect_verify_failure "$empty" truncate -s 0 "$empty/orza_${tag}_darwin_arm64.tar.gz"
duplicate=$TEST_TMPDIR/duplicate-checksum
expect_verify_failure "$duplicate" sh -c "printf '%s\n' \"\$(sed -n '1p' '$duplicate/orza_${tag}_checksums.txt')\" >>'$duplicate/orza_${tag}_checksums.txt'"

empty_binary=$TEST_TMPDIR/empty-binary
: >"$empty_binary"
if bash "$script_dir/package-release.sh" --tag "$tag" --target linux-amd64 --binary "$empty_binary" --output "$TEST_TMPDIR/rejected" >/dev/null 2>&1; then
  fail 'empty target binary was accepted'
fi

workflow=$(<"$script_dir/../.github/workflows/release.yml")
for target in "${targets[@]}"; do assert_contains "$workflow" "target: $target"; done
# shellcheck disable=SC2016
assert_contains "$workflow" 'RELEASE_VERSION: ${{ needs.eligibility.outputs.version }}'
# shellcheck disable=SC2016
assert_contains "$workflow" '-ldflags "-X main.version=$RELEASE_VERSION"'
assert_eq 1 "$(printf '%s\n' "$workflow" | grep -c 'go build -mod=vendor -trimpath -buildvcs=true')"

marker=$TEST_TMPDIR/hostile-side-effect
if bash "$script_dir/package-release.sh" --tag "v1.2.3;touch $marker" --target linux-amd64 \
  --binary "$TEST_TMPDIR/orza-linux-amd64" --output "$TEST_TMPDIR/hostile" >/dev/null 2>&1; then
  fail 'hostile tag was accepted'
fi
[ ! -e "$marker" ] || fail 'hostile package input caused a side effect'

printf 'PASS: deterministic six-target release packaging and verification\n'
