#!/usr/bin/env bash
set -euo pipefail

export GOTOOLCHAIN=local
exec python3 scripts/go-toolchain.py check
