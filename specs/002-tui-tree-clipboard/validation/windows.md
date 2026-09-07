# Windows Native Validation

**Status**: Pending native Windows amd64 and arm64 runners.

The Windows amd64 and arm64 Nix cross-builds pass. Native console validation is still required for the
three secret prompts, bracketed paste, Shift modifiers, alternate-handle fail-closed behavior,
`os.Interrupt` exit 130, context cancellation, terminal restoration, and the 20/20 keyboard sequence.
