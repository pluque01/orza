#!/usr/bin/env bash
set -euo pipefail

# Run from the repository root. An optional exact version also supports manual updates.
exec python3 scripts/go-toolchain.py update "$@"
