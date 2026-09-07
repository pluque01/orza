# Contract: GitHub Publication, Validation, and Releases

This contract defines observable repository behavior for feature 006. It does not change the application's CLI, TUI, SSH, catalog, or credential contracts.

## Public Repository Contract

Before changing visibility to public:

- Root `LICENSE` contains the canonical MIT terms for project-owned code and leaves vendored third-party notices intact.
- Root `README.md`, `SECURITY.md`, and `CONTRIBUTING.md` are present and linked where appropriate.
- The current publication tree is scanned for secrets with redacted output.
- If Git history exists or is imported, every available ref is scanned. If no history exists, the publication record says `not_present` rather than claiming a passed historical scan.
- Credible credentials are revoked or rotated before tree/history remediation is accepted.
- Repository metadata uses product name `Orza`, description `A terminal-native SSH client with a TUI`, and tagline `Navigate remote systems from your terminal.`; discovery topics identify an SSH client written in Go.
- Private vulnerability reporting, dependency graph/alerts, and available push protection are enabled.
- The `main` ruleset requires pull requests and the stable aggregate CI check, resolved conversations, and blocks force pushes and deletion.
- With one maintainer, required approvals are zero; when a second maintainer joins, required approvals become one and sensitive paths require code-owner review.

Authenticated settings that cannot be committed are recorded and verified with GitHub CLI/API output that contains no token or secret.

## Required CI Contract

### Events

| Event | Required behavior |
|-------|-------------------|
| Pull request | Validate the proposed merge revision on an ephemeral hosted runner with read-only permissions. |
| Push to `main` | Validate the exact resulting main revision. |
| Merge group | Run the same gate when merge queues are enabled. |
| Manual | Permit a maintainer to reproduce the same gate. |

No required workflow uses path filters. Superseded pull-request runs may be cancelled; main, merge-group, manual, and release runs are retained.

### Mandatory Checks

The stable aggregate required check passes only after:

1. Formatting succeeds for all project Go/Nix files.
2. The complete Go test suite succeeds.
3. The race-enabled suite succeeds.
4. `go vet` succeeds.
5. Staticcheck succeeds.
6. The pinned-database vulnerability policy succeeds.
7. Native package/build and existing smoke checks succeed.
8. `go mod tidy` plus vendor regeneration produce no repository difference.
9. Linux amd64/arm64 and Windows amd64/arm64 release outputs compile.
10. Secret scanning reports no unapproved current-tree/history finding.

`nix flake check --no-update-lock-file --keep-going -L` remains the local and CI source for checks 1-7. Independent jobs may expose granular diagnostics, but branch rules require one stable aggregate result.

### Security

- Default permissions are exactly `contents: read` or less.
- Checkout does not persist credentials into Git configuration when project code runs afterward.
- Pull-request jobs receive no release, cache-write, signing, or repository-administration secret.
- Public fork code never runs on a persistent self-hosted runner or under `pull_request_target` with write privileges.
- Every external action/reusable workflow uses a full 40-character commit SHA plus a readable release comment.
- Event fields enter command scripts through environment variables and quoted arguments, never direct command interpolation.

## Vulnerability Monitoring Contract

- Required CI uses the repository-locked vulnerability database for reproducibility.
- A separate scheduled/manual workflow queries current vulnerability data at least weekly; daily is permitted.
- Both paths use the same policy implementation and classify reachable IDs identically.
- Analysis/protocol failure, an unknown reachable ID, an expired exception, or a stale exception fails the job.
- Every active exception emits a visible warning naming only its ID, owner, issue, and expiration, without secret data.
- Informational unreachable findings remain visible but do not block and cannot satisfy a stale exception.

Exception JSON contract:

```json
[
  {
    "id": "GO-2026-5972",
    "owner": "pluque01",
    "rationale": "The locked Nix toolchain does not yet provide the fixed Go patch.",
    "mitigation": "Release builds use a fixed patch and the exception is reviewed on every database update.",
    "issue": "https://example.invalid/issues/1",
    "expires": "2026-10-04"
  }
]
```

The example is a schema illustration; implementation replaces the complete example record with a real reviewed issue, rationale, mitigation, owner, and expiration before accepting an exception.

The repository default is the empty JSON array in `.vulnerability-exceptions.json`. Its schema is equivalent to:

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "array",
  "maxItems": 10000,
  "items": {
    "type": "object",
    "additionalProperties": false,
    "required": ["id", "owner", "rationale", "mitigation", "issue", "expires"],
    "properties": {
      "id": { "type": "string", "pattern": "^GO-[0-9]{4}-[0-9]{4}$" },
      "owner": { "type": "string", "minLength": 1, "maxLength": 128 },
      "rationale": { "type": "string", "minLength": 1, "maxLength": 2048 },
      "mitigation": { "type": "string", "minLength": 1, "maxLength": 2048 },
      "issue": { "type": "string", "format": "uri", "pattern": "^https://" },
      "expires": { "type": "string", "format": "date" }
    }
  }
}
```

IDs must also be unique. Text fields reject leading/trailing-only content and control newlines. An expiration date is valid only when it is later than the UTC policy date. Unknown properties, wildcard IDs, and module-, severity-, or command-wide waivers are rejected.

## Stable Release Contract

### Trigger and Eligibility

- A pushed tag is eligible only if it exactly matches `vMAJOR.MINOR.PATCH`, with decimal components and no leading zero except `0`.
- The resolved tag commit must be contained in `origin/main`.
- The revision's required CI gate must pass or be rerun successfully before publication.
- A published release for the tag must not already exist.
- Same-tag runs share one non-cancelling concurrency group.

### Build Matrix

| OS | Architecture | Runner strategy | cgo | Archive |
|----|--------------|-----------------|------|---------|
| Linux | amd64 | Linux cross/native | disabled | `.tar.gz` |
| Linux | arm64 | Linux cross | disabled | `.tar.gz` |
| macOS | amd64 | Native Intel macOS | enabled | `.tar.gz` |
| macOS | arm64 | Native Apple-Silicon macOS | enabled | `.tar.gz` |
| Windows | amd64 | Linux cross | disabled | `.zip` |
| Windows | arm64 | Linux cross | disabled | `.zip` |

Every build uses exactly Go 1.26.6, `GOTOOLCHAIN=local`, committed vendor source, path trimming, VCS metadata, and version derived from the validated tag. The compile-only gate does not execute target binaries. A later Go patch requires one reviewed change to every visible builder pin.

Packaging tests assert that every matrix command receives the same validated linker version. A host-native post-public check may execute `--version` as evidence for FR-009, but it is not part of the six-target publication gate.

### Packaging

- Each archive contains exactly one executable at its root.
- Unix executables retain executable mode and are named `orza`; Windows uses `orza.exe`.
- Artifact names match the closed inventory in `data-model.md`.
- Intermediate workflow artifacts have one-day retention and fail when their expected file is absent.
- Build jobs have read-only permissions and cannot publish releases.

### Atomic Publication

1. A final job downloads all six archives.
2. It rejects missing, extra, duplicate, or zero-byte files.
3. It generates one filename-sorted SHA-256 manifest over final archive bytes.
4. It creates a draft release with generated notes and uploads exactly seven assets.
5. It queries the draft and verifies tag, target revision, draft state, asset inventory, release name/version, and generated change notes.
6. Only then, with job-local `contents: write`, it changes the draft to published.

Any failure before step 6 leaves no stable public release. A published release and its assets are never replaced by rerunning automation. Checksums detect corruption/differences but do not authenticate assets; signing and notarization remain out of scope.

## Dependency Update Contract

Renovate is the sole version-update proposal tool. It has one mutating scheduled workflow invocation each Monday UTC; manual runs are forced to full dry-run mode and cannot mutate branches or proposals. GitHub dependency alerts remain enabled, but Renovate vulnerability-alert PR mode and duplicate Dependabot update PRs are disabled so no proposal bypasses the global ceiling.

The mutating workflow uses one non-cancelling concurrency group and atomically creates the UTC ISO-week tag `automation/renovate/YYYY-Www` before Renovate runs. If that tag already exists, overlap and rerun attempts exit without mutation. The marker is retained on success or failure; failed cycles provide diagnostics and cannot be retried mutably until the next weekly key.

Only the trusted scheduled job receives a fine-grained token limited to this repository with metadata read and Contents, Pull requests, Issues, and Workflows read/write. Checkout credentials are not persisted, the token is masked, and logs never print environment/config values containing it. Pull-request and manual dry-run jobs receive no Renovate write token; manual discovery uses read-only repository access.

- Managers: Go modules, GitHub Actions, and Nix flakes/refs.
- Schedule: exactly one automatic invocation each Monday UTC.
- Open branch and PR ceilings: five repository-wide across every update class.
- Minimum release age: three days for proposals; urgent security findings remain visible as alerts while respecting the same proposal schedule and ceiling.
- Compatible updates: grouped only within the same ecosystem.
- Major/unknown-scheme updates: individual and require dependency-dashboard approval.
- Auto-merge: disabled for every update class.
- Go post-update: tidy and regenerate vendor source.
- Actions post-update: preserve full SHA and readable version comment.
- Nix: separate `nixpkgs` from vulnerability-database proposals; onboarding must prove recognition of the explicit revision or install a narrowly scoped reviewed updater.
- Every created proposal runs ordinary CI and follows the current maintainer approval policy.
- Each successful proposal lists old/new versions and every affected file; a deliberately failed dry run emits the dependency, cause, and evidence/reproduction location.

## Documentation Contract

Root README provides, in discoverable order:

1. Purpose, development status, MIT badge/link, CI status, and security warning boundary.
2. Supported OS/architecture matrix, VT terminal requirements, 40x12 minimum, English UI, no-color mode, and credential-store differences.
3. Release download naming and installation for Linux, macOS, and Windows.
4. SHA-256 verification commands and the non-authentication limitation.
5. Source/Nix build alternatives and exact local quality gate.
6. Version/help verification, synthetic folder/connection examples, keyboard TUI launch/navigation, and safe cancellation/exit.
7. Update and uninstall instructions that do not remove user catalog data unless explicitly requested.
8. Links to contribution and private security-reporting guidance.

All examples use `.invalid` hosts or explicitly synthetic names and never include passwords, private keys, real fingerprints, or production endpoints.

## Acceptance Matrix

| Area | Success | Principal failure proof |
|------|---------|-------------------------|
| Public readiness | Tree/history/license/docs/settings pass | Seeded secret blocks and is redacted |
| Pull-request CI | All required checks aggregate to pass | One controlled failure per category blocks |
| Vulnerability policy | No reachable finding or exact active exception | Unknown, expired, stale, malformed, and scanner errors fail |
| Tag eligibility | Exact tag on main passes | Invalid syntax, non-main commit, duplicate release fail |
| Release build | Six expected archives assemble | Any target failure prevents public release |
| Release publication | Verified seven-asset draft becomes public | Missing/extra/zero-byte/upload failure stays non-public |
| Dependency updates | Weekly compatible proposal with full CI | Major remains isolated; sixth branch/PR of any update class waits |
| Documentation | New user installs and opens help | Invalid platform/checksum path gives actionable correction |
