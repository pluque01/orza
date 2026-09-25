# Implementation Plan: Forget Host Key

**Branch**: `012-forget-host-key` | **Date**: 2026-09-25 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/012-forget-host-key/spec.md`

## Summary

Permit removing only Orza-owned trust for the host and port of a selected connection. Add a revision-protected deletion operation to the existing trusted-host repository, expose it through a dedicated application service, and make it available as a confirmed TUI action and as the automation-oriented `orza connection forget-host-key` command without a prompt. Neither path modifies OpenSSH `known_hosts` files.

## Technical Context

**Language/Version**: Go 1.26.0 (toolchain 1.26.6)

**Primary Dependencies**: Cobra CLI; Bubble Tea/Bubbles/Lip Gloss TUI; `golang.org/x/crypto/ssh` host-key support

**Storage**: Existing local SQLite `catalog.db`, specifically the existing `trusted_hosts` table and catalog revision

**Testing**: Go unit, package-conformance, CLI, TUI, and integration tests via `go test ./...`

**Target Platform**: Linux, macOS, and Windows terminal clients

**Project Type**: Terminal-first CLI with interactive TUI

**Performance Goals**: One bounded catalog read/delete transaction per request; no network operation, retry loop, or new background work

**Constraints**: Remove only app-owned host trust; leave standard `known_hosts` untouched; TUI requires `y` confirmation and must remain keyboard-operable; non-interactive command must not prompt

**Scale/Scope**: One endpoint-scoped trust record per canonical host and port; shared by every connection using that endpoint

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

**Pre-design gate: PASS**

- Secure SSH Operations: the feature only removes app-owned trust; it neither accepts a key nor relaxes `known_hosts` revocation or verification. The TUI action identifies the endpoint and requires confirmation.
- Terminal-First Usability: the TUI action, confirmation, cancellation, success, empty, and error states follow the existing keyboard-accessible modal patterns.
- Predictable State and Failure Handling: the deletion is conditional on the captured trusted-host revision. Missing or stale records cause a no-change outcome or conflict, and the TUI returns to a stable screen.
- Behavior-Focused Testing: repository, application, CLI, TUI confirmation, error, and concurrency paths receive automated coverage.
- Simplicity and Maintainability: reuse the existing repository, SQLite transaction, action registry, modal system, and output envelope; do not add dependencies, migrations, or background work.

**Post-design gate: PASS**

The phase-1 model and command contract preserve endpoint ownership, catalog revision semantics, standard SSH trust isolation, and the existing presentation boundaries. No constitutional exception or complexity justification is required.

## Project Structure

### Documentation (this feature)

```text
specs/012-forget-host-key/
├── plan.md              # This file (/speckit.plan command output)
├── research.md          # Phase 0 output (/speckit.plan command)
├── data-model.md        # Phase 1 output (/speckit.plan command)
├── contracts/
│   └── forget-host-key.md # CLI command and output contract
├── quickstart.md        # Phase 1 output (/speckit.plan command)
└── tasks.md             # Phase 2 output (/speckit.tasks command - NOT created by /speckit.plan)
```

### Source Code (repository root)
```text
cmd/orza/
└── main.go                         # Dependency composition

internal/
├── app/                            # Trust-management service, DTOs, and port
├── catalog/                        # Conditional trusted-host deletion
├── hostkey/                        # Catalog/app trust adapter
├── cli/                            # connection forget-host-key command and tests
└── tui/                            # Connection action, modal, workflow, and tests

tests/integration/                  # End-to-end CLI/TUI behavior where needed
```

**Structure Decision**: Keep the existing single Go module and layered presentation/application/catalog design. Add only focused types and operations in the existing packages; the TUI does not access catalog storage directly.

## Complexity Tracking

No constitutional violations or complexity exceptions.
