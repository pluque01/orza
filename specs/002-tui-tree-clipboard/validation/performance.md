# Performance Validation

**Date**: 2026-07-31
**Host**: Linux amd64, AMD Ryzen 5 7600X, 12 logical CPUs, 16.3 GB RAM (14.4 GB available)
**Toolchain**: Go 1.26.5 through `nix develop`, Nix 2.34.8
**Fixture**: 100 folders, 1,000 connections, depth 10, synthetic 80x24 view, no concurrent test load
**Command**: `nix develop -c go test ./internal/tui -run '^TestTreePerformanceAcceptance$' -count 1 -v`

Each operation performed one unmeasured warm-up followed by 20 measured runs. Qualification requires at
least 19 runs below one second.

## Refresh

| Run | Duration | Qualified |
|---:|---:|:---:|
| 1 | 1.988157 ms | yes |
| 2 | 422.372 us | yes |
| 3 | 850.158 us | yes |
| 4 | 461.385 us | yes |
| 5 | 476.193 us | yes |
| 6 | 654.232 us | yes |
| 7 | 294.219 us | yes |
| 8 | 990.446 us | yes |
| 9 | 390.892 us | yes |
| 10 | 838.366 us | yes |
| 11 | 227.642 us | yes |
| 12 | 708.601 us | yes |
| 13 | 301.533 us | yes |
| 14 | 572.608 us | yes |
| 15 | 231.750 us | yes |
| 16 | 393.557 us | yes |
| 17 | 743.170 us | yes |
| 18 | 305.059 us | yes |
| 19 | 821.218 us | yes |
| 20 | 266.836 us | yes |

**Result**: 20/20 qualified.

## Expand

| Run | Duration | Qualified |
|---:|---:|:---:|
| 1 | 1.282 us | yes |
| 2 | 962 ns | yes |
| 3 | 921 ns | yes |
| 4 | 922 ns | yes |
| 5 | 942 ns | yes |
| 6 | 941 ns | yes |
| 7 | 922 ns | yes |
| 8 | 932 ns | yes |
| 9 | 982 ns | yes |
| 10 | 951 ns | yes |
| 11 | 962 ns | yes |
| 12 | 922 ns | yes |
| 13 | 922 ns | yes |
| 14 | 952 ns | yes |
| 15 | 922 ns | yes |
| 16 | 1.252 us | yes |
| 17 | 1.022 us | yes |
| 18 | 922 ns | yes |
| 19 | 931 ns | yes |
| 20 | 932 ns | yes |

**Result**: 20/20 qualified.

## Collapse

| Run | Duration | Qualified |
|---:|---:|:---:|
| 1 | 1.102 us | yes |
| 2 | 1.042 us | yes |
| 3 | 1.002 us | yes |
| 4 | 972 ns | yes |
| 5 | 1.002 us | yes |
| 6 | 982 ns | yes |
| 7 | 972 ns | yes |
| 8 | 982 ns | yes |
| 9 | 962 ns | yes |
| 10 | 972 ns | yes |
| 11 | 1.022 us | yes |
| 12 | 972 ns | yes |
| 13 | 992 ns | yes |
| 14 | 1.001 us | yes |
| 15 | 972 ns | yes |
| 16 | 972 ns | yes |
| 17 | 992 ns | yes |
| 18 | 981 ns | yes |
| 19 | 982 ns | yes |
| 20 | 982 ns | yes |

**Result**: 20/20 qualified.

## Go Benchmarks

Command: `nix develop -c go test ./internal/tui -run '^$' -bench '^BenchmarkTree(Refresh|Expand|Collapse)$' -benchtime=20x -count=1`

```text
BenchmarkTreeRefresh-12       20  502466 ns/op
BenchmarkTreeExpand-12        20    1312 ns/op
BenchmarkTreeCollapse-12      20     974.4 ns/op
```

The acceptance threshold passed, so the conditional aggregated catalog snapshot was not needed.
