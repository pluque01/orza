# Phase 0 Research: Publicar el proyecto en GitHub

## Reproducible CI Contract

**Decision**: Keep `nix flake check --no-update-lock-file --keep-going -L` as the authoritative required CI command. Add explicit vendor-regeneration and release-output checks rather than duplicating each Go command in workflow YAML.

**Rationale**: The flake already pins Go, formatting, tests, race, vet, Staticcheck, vulnerability data, build, and native smoke behavior. Reusing it keeps local and hosted validation identical. Vendor regeneration and all cross outputs are properties the current aggregate check does not prove.

**Alternatives considered**:

- Recreate every check with separate hosted actions: rejected because local/CI behavior would drift and more tool versions would need independent pinning.
- Run only direct Go commands: rejected because it loses the reviewed Nix toolchain and cross-package contract.
- Add a shared binary cache immediately: deferred until measured runtimes justify cache trust, storage, and maintenance costs.

## CI Events, Permissions, and Concurrency

**Decision**: Run required CI for pull requests, pushes to `main`, optional merge-group checks, and manual diagnosis. Use ephemeral hosted runners, top-level `contents: read`, checkout without persisted credentials, no path filters, and cancellation only for superseded pull-request runs.

**Rationale**: Pull requests from forks execute untrusted code. Read-only tokens and hosted runners contain risk; avoiding `pull_request_target`, secrets, persistent runners, and direct interpolation of GitHub context prevents common privilege-escalation paths. Stable aggregate job names support branch rules without ambiguity.

**Alternatives considered**:

- Run on every branch push: rejected because it duplicates pull-request work.
- Use path filters: rejected because skipped required workflows can remain pending and automation/Nix changes have broad impact.
- Cancel all previous runs: rejected because `main` and release evidence should remain complete.

## Immutable External Automation

**Decision**: Pin every external action and reusable workflow to a full commit SHA with a readable release comment, and let the dependency updater maintain those pins. Pin the Nix installer version as well as its action.

**Rationale**: GitHub documents a full-length commit SHA as the only immutable action reference. A version comment preserves reviewability and enables automated updates.

**Alternatives considered**:

- Major or floating tags: rejected because a third party can move them after review.
- Commit SHAs without release comments: rejected because reviewers cannot easily identify the intended upstream release.

## Vulnerability Freshness and Exceptions

**Decision**: Keep the locked vulnerability database for reproducible required CI and add a scheduled live-database scan. Both invoke a tested policy command that parses `govulncheck` streaming JSON, fails on analysis/protocol errors and every non-excepted reachable ID, warns for active exceptions, and rejects expired or stale exception records.

Each exception contains the exact `GO-YYYY-NNNN` ID, owner, rationale, mitigation, tracking issue, and ISO expiration/review date. Normal validity is 7-30 days.

**Rationale**: The locked database makes pull-request results reproducible but cannot discover advisories published after `flake.lock`. `govulncheck` has no native expiring per-ID suppression; `continue-on-error`, text matching, or unparsed JSON would silently waive unrelated vulnerabilities. The current Go 1.26.5 `GO-2026-5972` limitation demonstrates the need for an explicit temporary policy until the pinned environment supplies Go 1.26.6.

**Alternatives considered**:

- Make all vulnerability output advisory: rejected because reachable vulnerabilities must block by default.
- Use only the live database in required CI: rejected because results can change without a repository change.
- Match text output with shell tools: rejected as brittle and unable to validate exception lifecycle reliably.

## Release Builder Strategy

**Decision**: Build releases directly with exactly Go 1.26.6, vendored modules, `GOTOOLCHAIN=local`, `-trimpath`, VCS stamping, and linker version injection. Any later patch upgrade is one reviewed repository change that updates every builder pin together. Use Linux for pure-Go Linux/Windows cross-builds and native Intel/Apple-Silicon macOS runners with cgo enabled for Darwin.

**Rationale**: The project has platform-specific credential backends. Darwin requires cgo to select and link the real Keychain implementation; a Linux-hosted pure-Go Darwin cross-build would silently ship the unavailable credential-store fallback. Direct Go avoids the flake's static development version and an additional release framework while satisfying the compile-only gate.

**Alternatives considered**:

- Build all six outputs on Linux: rejected because Darwin cgo/Keychain would be incorrect.
- Use the current Nix outputs for releases: rejected for the initial release because Darwin still needs two native builders and the flake version is static.
- Add GoReleaser: rejected because native split/merge cgo publication adds configuration or paid features for only six straightforward artifacts.

## Tag and Main-Branch Eligibility

**Decision**: Trigger on tag-like refs, then fail unless the tag exactly matches `^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$` and its commit is an ancestor of `origin/main`. Pass the tag through a quoted environment variable rather than embedding event expressions in shell code.

**Rationale**: GitHub tag filters are globs, not semantic-version validators. An ancestry check permits any already-integrated main commit while excluding unreviewed branches. Quoted environment transport treats tags as untrusted input.

**Alternatives considered**:

- Compare only with current `main` HEAD: rejected because valid older integrated commits could not be released.
- Allow any validated commit: rejected by the clarification requiring integration into `main`.
- Use tag text directly in commands: rejected because event values are untrusted.

## Artifact Packaging and Atomic Publication

**Decision**: Produce `.tar.gz` archives for Linux/macOS and `.zip` archives for Windows. Name them `orza_vX.Y.Z_<os>_<arch>.<ext>`, each containing exactly `orza` or `orza.exe`. Generate one C-locale-sorted SHA-256 manifest over the six archives. Assemble and verify the exact non-empty seven-file inventory before creating a draft, upload all assets, verify the draft, then publish it. Never replace a published release.

**Rationale**: Native archive formats preserve executable mode where needed and are familiar to users. Draft-first publication means incomplete builds or uploads never appear as stable releases. Hashing final archives verifies the bytes users download.

**Alternatives considered**:

- Publish each matrix job directly: rejected because partial releases become visible and write permission spreads across untrusted build jobs.
- Hash unpackaged binaries: rejected because those hashes would not verify downloaded archives.
- Add signing/notarization now: excluded by specification until signing identities and secrets exist.

## Dependency Update Tool

**Decision**: Use a Nix-pinned Renovate CLI for version proposals and retain GitHub dependency graph/alerts for vulnerability visibility. Invoke mutating Renovate from one scheduled GitHub workflow every Monday UTC. Serialize it in one non-cancelling concurrency group and atomically create `automation/renovate/YYYY-Www` before invoking Renovate; an existing week tag makes every overlap/rerun exit without mutation. The week tag remains even if Renovate fails, so diagnosis is manual/read-only and mutation resumes next week. Use a fine-grained repository token only in that trusted scheduled job; manual invocations are forced to full dry-run mode and never receive it. Set repository-wide `prConcurrentLimit: 5` and `branchConcurrentLimit: 5`; disable Renovate vulnerability-alert PR creation and Dependabot update PRs so no proposal class bypasses the ceiling; regenerate `go.mod`, `go.sum`, and `vendor/`; group non-major updates only within an ecosystem; isolate majors behind dashboard approval; disable auto-merge; and apply a three-day minimum release age.

**Rationale**: Renovate provides repository-wide branch/PR ceilings, full-SHA action pinning, vendoring hooks, and broader Nix/ref support. A repository-owned weekly invocation makes the one-cycle cadence testable; a hosted app may scan or update more than once inside a schedule window. GitHub alerts remain the urgent signal while the next bounded cycle proposes corrections. Dependabot limits are per ecosystem and would not reliably update the explicit vulnerability-database revision in `flake.nix`.

The fine-grained token is scoped only to this repository with metadata read plus Contents, Pull requests, Issues, and Workflows read/write, which Renovate needs to update dependency files/workflows, maintain its dashboard, and open proposals. It is exposed only to the scheduled Renovate step after trusted `main` checkout with persisted checkout credentials disabled, is masked in logs, and is never available to pull-request or manual dry-run jobs.

**Alternatives considered**:

- Renovate hosted App: rejected because its background invocation cadence cannot prove exactly one proposal cycle per week.
- Dependabot version PRs: simpler first-party operation, but rejected because three ecosystems can exceed the global five-PR requirement and explicit Nix refs need separate handling.
- Run both tools for updates: rejected because duplicate/conflicting proposals would result.
- Auto-merge compatible updates: rejected by the explicit maintainer-review requirement.

## Nix Update Boundary

**Decision**: Keep `nixpkgs` and `vulndb` updates separate. Validate Renovate's beta Nix manager during onboarding; if it cannot update the explicit `vulndb` revision and associated fixed-output hash, use a narrowly scoped custom manager or reviewed scheduled updater rather than silently omitting that input.

**Rationale**: Updating vulnerability data can also change the indexer's dependency hash. Combining it with unrelated platform updates obscures failures and may leave security data stale.

**Alternatives considered**:

- Group all flake inputs: rejected because toolchain and vulnerability-data changes have different risk and remediation.
- Leave the database pinned indefinitely: rejected because scheduled live findings would diverge from required CI permanently.

## Pre-Publication Secret Review

**Decision**: Before the first public push, scan the current tree and every available Git ref with redacted output. If an existing history is imported, scan all refs and handle LFS/submodules separately. Revoke or rotate credible credentials before removing them. If no history exists, record that fact, scan the complete initial tree, create a new history only after the scan passes, and scan the newly created history to a `passed` state before changing visibility.

**Rationale**: The current workspace has no `.git`, so claiming that its history is clean would be false. Deleting a secret from the current tree does not neutralize a credential in imported history.

**Alternatives considered**:

- Scan only the current files: rejected when history exists because removed secrets remain public after push.
- Rewrite history without rotating credentials: rejected because exposed credentials remain usable.

## Public Repository Governance

**Decision**: Add root MIT, security, contribution, and ownership documents. Configure a GitHub ruleset requiring pull requests, stable CI, resolved conversations, no force pushes/deletion, and zero external approvals while one maintainer exists. When a second maintainer joins, require one other-person approval and code-owner review for security-sensitive paths. Enable private vulnerability reporting and repository security alerts.

**Rationale**: Requiring one approval now would deadlock the sole maintainer. A ruleset preserves audited controls and supports a later policy transition. Private reporting prevents forced public disclosure.

**Alternatives considered**:

- Require approval immediately: rejected by the sole-maintainer clarification.
- Allow direct unvalidated pushes: rejected because it bypasses the publication quality gate.
- Use only informal documentation: rejected because GitHub settings enforce the expected integration behavior.

## Primary References

- GitHub Actions workflow syntax and permissions: https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-syntax
- GitHub Actions security hardening: https://docs.github.com/en/actions/reference/security/secure-use
- GitHub workflow concurrency: https://docs.github.com/en/actions/how-tos/write-workflows/choose-when-workflows-run/control-workflow-concurrency
- GitHub releases: https://docs.github.com/en/repositories/releasing-projects-on-github/managing-releases-in-a-repository
- GitHub CLI release creation: https://cli.github.com/manual/gh_release_create
- GitHub repository rulesets: https://docs.github.com/en/repositories/configuring-branches-and-merges-in-your-repository/managing-rulesets/creating-rulesets-for-a-repository
- GitHub private vulnerability reporting: https://docs.github.com/en/code-security/security-advisories/working-with-repository-security-advisories/configuring-private-vulnerability-reporting-for-a-repository
- Go module vendoring: https://go.dev/ref/mod#vendoring
- Go vulnerability management: https://go.dev/doc/security/vuln/
- Renovate Go manager: https://docs.renovatebot.com/modules/manager/gomod/
- Renovate GitHub Actions manager: https://docs.renovatebot.com/modules/manager/github-actions/
- Renovate Nix manager: https://docs.renovatebot.com/modules/manager/nix/
- Gitleaks: https://github.com/gitleaks/gitleaks
- MIT license: https://opensource.org/license/mit
