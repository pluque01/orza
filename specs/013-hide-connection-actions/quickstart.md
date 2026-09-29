# Quickstart: Validate Hidden Actions Panel

## Prerequisites

- A development environment with the project dependencies available through Nix.
- A terminal at least 40 columns by 12 rows for normal interactive layout validation.

## Automated Validation

Run the complete suite:

```bash
nix develop --command go test ./...
```

Expected outcome: all unit, conformance, resize, accessibility, and integration tests pass.

## Manual Validation

1. Launch the TUI and inspect catalog browsing.
   Expected: no titled or bordered Actions panel; the browser action legend remains visible.
2. Open a create or edit form.
   Expected: no Actions panel; the form action legend remains visible in the fixed lower region.
3. Start a connection, including a host identity verification path if available.
   Expected: no Actions panel; startup status, trust controls, and cancel or quit controls remain visible as applicable.
4. Cancel or cause a recoverable startup failure.
   Expected: no Actions panel appears; the appropriate recovery or catalog action legend remains visible.
5. Repeat the relevant scenarios at 40x12.
   Expected: the interface remains readable and keyboard-operable without reassigning the former lower control region.

See [tui-presentation.md](contracts/tui-presentation.md) for the state-by-state presentation contract and [data-model.md](data-model.md) for presentation invariants.
