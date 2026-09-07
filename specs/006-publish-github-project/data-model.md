# Data Model: Publicar el proyecto en GitHub

This feature adds no application-domain or SQLite entities. The following repository and automation entities are persisted as reviewed configuration or derived during one workflow run.

## Repository Readiness

Represents whether the source can safely and legally become public.

| Field | Type | Rules |
|-------|------|-------|
| `sourceReview` | enum | `pending`, `passed`, or `failed`; covers current tracked/untracked publication tree. |
| `historyReview` | enum | `not_present`, `pending`, `passed`, or `failed`; `not_present` is valid only before initialization when `.git` does not exist. |
| `license` | fixed string | `MIT` for project-owned source. |
| `ownerAuthorized` | boolean | Must be true before changing repository visibility. |
| `metadataReady` | boolean | Product name `Orza`, description `A terminal-native SSH client with a TUI`, tagline `Navigate remote systems from your terminal.`, topics, README, security policy, and contribution guidance are present. |
| `rulesReady` | boolean | Main ruleset reflects the current maintainer count and required CI gate. |
| `securityReady` | boolean | Private reporting, dependency alerts, and supported security controls are enabled or their platform limitation is documented. |

### Invariants

- Public visibility requires `sourceReview=passed`, `historyReview=passed`, `license=MIT`, and all readiness booleans true.
- `historyReview=not_present` cannot be represented as `passed`; after Git initialization it transitions to `pending` and then `passed` or `failed`.
- A project that begins with `historyReview=not_present` must scan the newly initialized history and reach `passed` before public visibility.
- Any credible secret finding changes the corresponding review to `failed`; the secret is revoked/rotated before history or tree remediation is considered complete.
- Scanner output stored in logs is redacted and contains no recovered secret value.

## Validation Run

One CI evaluation associated with a pull request, `main` commit, merge group, or manual request.

| Field | Type | Rules |
|-------|------|-------|
| `event` | enum | `pull_request`, `main_push`, `merge_group`, or `manual`. |
| `revision` | commit identifier | Immutable source revision under test. |
| `checks` | set of Validation Result | Exactly the required categories for the event. |
| `aggregateStatus` | enum | `pending`, `passed`, `failed`, or `cancelled`. |
| `permissions` | permission map | Read-only by default; CI has no write permission. |

### Validation Result

| Field | Type | Rules |
|-------|------|-------|
| `name` | stable string | Unique required-check name used by repository rules. |
| `category` | enum | `format`, `test`, `race`, `vet`, `static`, `vulnerability`, `build`, `vendor`, or `secret`. |
| `target` | optional platform/architecture | Present for target-specific builds. |
| `status` | enum | `pending`, `passed`, `failed`, `cancelled`, or `excepted`. |
| `diagnostic` | optional Diagnostic | Required for `failed` and `excepted`. |

### State Transitions

```text
pending -> passed
pending -> failed
pending -> cancelled       (superseded pull-request run only)
pending -> excepted        (reachable vulnerability with active exception only)
```

The aggregate passes only when every required result is `passed` or the vulnerability result is `excepted` by a valid record. Cancelled main, merge-group, manual, and release evidence is not accepted as passed.

## Diagnostic

Bounded information needed to investigate a failure without exposing secrets.

| Field | Type | Rules |
|-------|------|-------|
| `check` | string | Validation or release step that failed. |
| `target` | optional string | Platform, architecture, dependency, or vulnerability ID. |
| `cause` | sanitized text | Controlled or tool-produced summary with secret redaction. |
| `evidenceLocation` | URL or command | Workflow log location or local reproduction command. |

## Vulnerability Exception

Reviewed temporary policy record stored in `.vulnerability-exceptions.json`.

| Field | Type | Rules |
|-------|------|-------|
| `id` | string | Exact `GO-YYYY-NNNN`; unique in the file. |
| `owner` | non-empty string | Accountable maintainer. |
| `rationale` | non-empty string | Why a compatible correction is unavailable. |
| `mitigation` | non-empty string | Temporary risk reduction. |
| `issue` | absolute HTTPS URL | Public tracking issue without secret detail. |
| `expires` | ISO date | Date after which the exception fails closed. |

### Validation

- Unknown fields, duplicate IDs, malformed IDs/URLs/dates, empty required text, and expired records are invalid.
- A finding is excepted only by an exact ID match and only while the record is valid.
- A valid record whose finding has disappeared is stale and fails, forcing removal.
- Scanner protocol/analysis errors always fail and cannot be excepted.
- Non-reachable informational module findings are reported but do not consume exception records.

## Release

Represents one stable GitHub release and its atomic lifecycle.

| Field | Type | Rules |
|-------|------|-------|
| `tag` | string | Exact `vMAJOR.MINOR.PATCH`, no leading zeros except zero. |
| `version` | string | Tag without `v`; injected into every binary. |
| `revision` | commit identifier | Must be contained in `origin/main`. |
| `state` | enum | `requested`, `validated`, `building`, `assembled`, `draft`, `published`, or `failed`. |
| `artifacts` | set of Release Artifact | Exactly six archives before publication. |
| `checksumManifest` | Release Artifact | Exactly one sorted SHA-256 manifest. |
| `notes` | text | Generated from repository changes for this tag. |

### State Transitions

```text
requested -> validated -> building -> assembled -> draft -> published
     |            |           |           |         |
     +------------+-----------+-----------+---------+-> failed
```

- `validated` requires exact tag syntax, main ancestry, no existing published release, and a passing mandatory validation gate for the revision.
- `assembled` requires all six non-empty archives and the checksum manifest.
- `draft` is never presented as a stable public release.
- `published` is terminal and immutable; reruns cannot replace its assets.
- A stale draft for the same tag may be deleted and recreated only after confirming no public release exists.
- Concurrent runs for the same tag are serialized and never cancelled mid-publication.

## Release Artifact

| Field | Type | Rules |
|-------|------|-------|
| `project` | fixed string | `orza`. |
| `tag` | string | Same as parent release tag. |
| `os` | enum | `linux`, `darwin`, or `windows`; absent only for manifest. |
| `arch` | enum | `amd64` or `arm64`; absent only for manifest. |
| `format` | enum | `tar.gz` for Linux/Darwin, `zip` for Windows, `txt` for manifest. |
| `filename` | string | Derived exactly from project, tag, OS, architecture, and format. |
| `executable` | string | Exactly `orza` or `orza.exe` inside an archive. |
| `size` | positive integer | Zero-byte assets are rejected. |
| `sha256` | 64 lowercase hexadecimal characters | Hashes final archive bytes. |

### Closed Inventory

For `v1.2.3`, the seven assets are:

```text
orza_v1.2.3_linux_amd64.tar.gz
orza_v1.2.3_linux_arm64.tar.gz
orza_v1.2.3_darwin_amd64.tar.gz
orza_v1.2.3_darwin_arm64.tar.gz
orza_v1.2.3_windows_amd64.zip
orza_v1.2.3_windows_arm64.zip
orza_v1.2.3_checksums.txt
```

## Dependency Update Proposal

One Renovate-managed pull request.

| Field | Type | Rules |
|-------|------|-------|
| `ecosystem` | enum | `gomod`, `github-actions`, or `nix`. |
| `packages` | non-empty set | Every package has current and proposed versions/revisions. |
| `affectedFiles` | non-empty set of paths | Lists every manifest, lock, vendor, workflow, or policy file changed by the proposal. |
| `updateClass` | enum | `compatible`, `major`, `digest`, or `security`. |
| `group` | optional string | Compatible updates only, within one ecosystem. |
| `status` | enum | `proposed`, `validated`, `blocked`, `approved`, `merged`, or `closed`. |
| `automerged` | fixed boolean | Always false. |
| `diagnostic` | optional Diagnostic | Required when proposal generation or validation is blocked. |

### Invariants

- At most five Renovate branches and five update proposals are open repository-wide across every update class.
- Vulnerability-alert and other bypass proposal modes are disabled; GitHub security alerts remain visible without creating a sixth update PR.
- Major and version-scheme-unknown updates are separate and require dashboard approval before proposal.
- Go proposals regenerate `go.mod`, `go.sum`, `vendor/`, and `vendor/modules.txt` coherently.
- Action proposals preserve full-SHA pins and update readable release comments.
- `nixpkgs` and vulnerability-database changes remain separate.
- Every proposal uses the ordinary pull-request CI and current maintainer approval policy.

## Relationships

- A Repository Readiness result gates the first public repository transition.
- A Validation Run evaluates one revision and gates integration or Release validation.
- A Vulnerability Exception may justify only matching vulnerability results within Validation Runs.
- A Release owns exactly six platform artifacts and one checksum manifest.
- A Dependency Update Proposal creates an ordinary Validation Run and can create a releasable revision only after merge to `main`.
