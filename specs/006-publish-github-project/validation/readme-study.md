# README Usability Study Protocol

## Status And Privacy

**Status:** waived by the repository owner on 2026-09-08. The study was not run and this record makes
no claim about participant results or usability thresholds.

Recruit exactly 10 people who did not implement feature 006. Randomly assign study IDs `P01` through
`P10`; do not record names, contact details, employer, account names, IP addresses, SSH endpoints,
credentials, fingerprints, terminal recordings, or free-form personal data. Record only the assigned
ID, platform path, elapsed times, task correctness, observed documentation blocker category, and
completion status. Use synthetic local data and release fixtures only.

The group must exercise at least one Linux, one macOS, and one Windows path, with both architectures
covered where suitable test machines or asset-selection scenarios are available. A facilitator may
provide a clean machine or disposable VM and the root `README.md`, but no other project document or
instruction.

## Procedure

1. Start one 15-minute timer when the participant receives only `README.md` and a stated OS/architecture.
2. Ask the participant to select the exact matching `orza_v...` release asset and checksum manifest.
3. Ask them to explain what SHA-256 does and does not establish, then use the documented platform command to verify the fixture.
4. Ask them to install the executable, run `orza --version`, and open `orza --help`. Record the elapsed time when all three are complete.
5. Ask them to use only synthetic `.invalid` data to create folders and a connection, inspect and move it, launch the TUI, identify keyboard navigation, cancel/back, and safe exit without connecting.
6. Ask them to locate the exact local quality gate, stable release trigger, six release targets, one release-blocking condition, update command, and executable-only removal command.
7. Stop at 15 minutes. Record correctness using the rubric below; do not coach during a timed task. After timing ends, remove the fixture executable and synthetic records without deleting or inspecting unrelated catalog data.

## Correctness Rubric

Mark each item `pass`, `fail`, or `not attempted`; do not infer success from another item.

| Item | Pass condition |
|---|---|
| Asset selection | Exact archive matches stated OS and architecture. |
| Integrity boundary | Runs the documented SHA-256 command and states that a matching checksum detects changed bytes but does not authenticate the publisher/artifact. |
| Install/version/help | Installs `orza` or `orza.exe`, obtains the release version, and opens help within 10 minutes. |
| Synthetic CLI | Uses `.invalid` metadata and completes documented create, show/list, update, and move operations without network activity. |
| TUI safety | Identifies keyboard selection/expansion, Help, cancel/back, and safe `q` or `Ctrl-C` exit. |
| Contributor discovery | Finds `nix flake check --no-update-lock-file --keep-going -L`. |
| Release discovery | Identifies exact `vMAJOR.MINOR.PATCH`, all six OS/architecture targets, and a blocking CI/build/asset condition within 15 minutes. |
| Lifecycle | Finds update and executable-removal commands and does not delete catalog data. |

The acceptance thresholds, evaluated only after all records are complete, are at least 9 of 10 for
install/version/help within 10 minutes, and every participant correctly identifying all six targets,
the exact release trigger, and a blocking validation within 15 minutes. Any unmet threshold remains a
documentation finding; it must not be rewritten as a pass.

## Participant Records

Leave cells empty until an actual session occurs. Times are elapsed `mm:ss`; blocker category is one
of `none`, `asset`, `checksum`, `install`, `PATH`, `CLI`, `TUI`, `release`, or `other` without free-form
personal detail.

| ID | Platform/arch | Install + version + help time | Asset | Integrity | CLI | TUI | Gate/trigger/targets | Lifecycle | Completion | Blocker category |
|---|---|---|---|---|---|---|---|---|---|---|
| P01 | | | | | | | | | | |
| P02 | | | | | | | | | | |
| P03 | | | | | | | | | | |
| P04 | | | | | | | | | | |
| P05 | | | | | | | | | | |
| P06 | | | | | | | | | | |
| P07 | | | | | | | | | | |
| P08 | | | | | | | | | | |
| P09 | | | | | | | | | | |
| P10 | | | | | | | | | | |

## Waived Aggregate Record

No participant rows were completed. Blank numerators are not zero and are not results. The owner
accepted proceeding without the human usability evidence required by SC-005 and the timed-newcomer
portion of SC-009; automated and static README checks remain applicable.

| Field | Value |
|---|---|
| Study state | `waived by owner; not run` |
| Sessions completed | `0; no outcome inferred` |
| Linux / macOS / Windows paths completed | `not measured` |
| Install + version + help within 10 minutes | `not measured` |
| Correct trigger, six targets, and blocker within 15 minutes | `not measured` |
| Checksum limitation identified | `not measured` |
| Safe TUI cancellation/exit identified | `not measured` |
| Catalog-preserving removal identified | `not measured` |
| Acceptance outcome | `waived; unverified` |
| Documentation blocker categories and counts | `not measured` |
