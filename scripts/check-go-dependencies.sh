#!/usr/bin/env bash
set -euo pipefail

# Works in a writable sandbox source copy; no Git checkout or network is needed.
temporary=$(mktemp -d)
trap 'rm -rf "$temporary"' EXIT HUP INT TERM
cp go.mod go.sum "$temporary/"
export GOTOOLCHAIN=local GOPROXY=off GOSUMDB=off
go mod tidy
cmp go.mod "$temporary/go.mod" || { printf 'dependencies: go mod tidy changed go.mod\n' >&2; exit 1; }
cmp go.sum "$temporary/go.sum" || { printf 'dependencies: go mod tidy changed go.sum\n' >&2; exit 1; }
go mod verify
printf 'dependencies: manifests are tidy and module contents verified offline\n'
