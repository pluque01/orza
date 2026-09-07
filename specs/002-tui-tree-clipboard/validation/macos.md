# macOS Native Validation

**Status**: Pending native macOS amd64 and arm64 runners.

Native validation is required for the three secret prompts, bracketed paste, Shift modifiers,
SIGINT/SIGTERM exit 130/143, cancellation, terminal restoration, and the 20/20 keyboard sequence. The
Linux host cannot build or execute the Darwin-native checks without a configured Darwin builder.
