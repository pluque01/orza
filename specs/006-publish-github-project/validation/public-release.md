# Public Release Validation

**Date:** 2026-09-08
**Repository:** `https://github.com/pluque01/orza`
**Release:** `https://github.com/pluque01/orza/releases/tag/v0.1.0`
**Revision:** `316f15b8db11152af3df919c47521e777166d7ec`

## Authorization And Visibility

- The repository owner confirmed ownership, MIT distribution of the project's code and artifacts,
  and explicitly authorized public visibility plus release `v0.1.0`.
- The repository was changed from private to public only after the final tree/all-ref scan and CI run
  `34273542742` passed for the exact release revision.
- An unauthenticated HTTPS clone succeeded at the expected revision and contained `LICENSE`,
  `README.md`, and `SECURITY.md`.
- The unauthenticated API returned the public repository. Description, topics, MIT license, selected
  full-SHA Actions policy, read-only workflow defaults, and the active no-bypass `Protect main`
  ruleset passed `scripts/verify-github.sh --expect-visibility public`.
- Secret Scanning, Push Protection, dependency alerts, and Private Vulnerability Reporting are enabled.

## Release Workflow

- Lightweight tag `v0.1.0` points to the validated `main` revision above.
- Release run `34275131223` passed eligibility, all six compilation jobs, exact assembly, draft
  verification, and final publication.
- Linux and Windows amd64/arm64 outputs compiled with the pinned Go toolchain. Darwin amd64/arm64
  outputs compiled on matching native macOS runners with cgo enabled.
- The release is public, non-draft, non-prerelease, named `Orza v0.1.0`, targets the exact revision,
  and contains generated notes identifying the included changes.

## Public Asset Inventory

| Asset | Bytes | SHA-256 |
|---|---:|---|
| `orza_v0.1.0_darwin_amd64.tar.gz` | 11996688 | `fc9219c648a1a6a1b0bda9c20be95f1644a16b272360c828e26fa19189bd3884` |
| `orza_v0.1.0_darwin_arm64.tar.gz` | 11281162 | `1294c0e301cca8574b6fd9fa5dbac21b1d145b80fd2055f0e4ebd2b924c846b6` |
| `orza_v0.1.0_linux_amd64.tar.gz` | 11369825 | `7b7f7d3676ff8d8906be98035e60d04c29e9b364a0e5953279f8ba091bc7d324` |
| `orza_v0.1.0_linux_arm64.tar.gz` | 10633580 | `20bed40ddd51af003d8d15b0abc530eff0e13b9e85ea43bad6f8ed36deb149dc` |
| `orza_v0.1.0_windows_amd64.zip` | 11470303 | `3c6e2af3df5d76fa6643ae9333067936c6b1b79dd34dd335dcb73ebb655061a7` |
| `orza_v0.1.0_windows_arm64.zip` | 10664345 | `7d17280a42ac44610ae78716e1fca5bb73d62b83b6bd2884821487d6c1608194` |
| `orza_v0.1.0_checksums.txt` | 582 | `affff119adcc2aef32d447abc235958aaef82d6db00dea458d012f5294922325` |

All six archive checks passed against the downloaded checksum manifest. The downloaded Linux amd64
archive contained the expected executable, and the compatible host execution returned
`orza 0.1.0`.

## Replacement Boundary

The publication workflow refuses to replace an existing published release for the same tag, and the
recorded asset digests make later byte changes detectable. GitHub's separate native immutable-release
field reports `false` for `v0.1.0`; that feature only applies to releases created after it is enabled,
so native GitHub immutability is not claimed for this release.

SC-002, SC-003, and SC-010 are satisfied by the public evidence above. SC-005 and the timed portion
of SC-009 remain explicitly waived/unverified as recorded in `readme-study.md` and `acceptance.md`.
