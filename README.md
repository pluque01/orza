# Orza

[![CI](https://github.com/pluque01/orza/actions/workflows/ci.yml/badge.svg)](https://github.com/pluque01/orza/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/pluque01/orza)](https://github.com/pluque01/orza/releases)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Navigate remote systems from your terminal.

Orza is a terminal-first SSH connection manager. Organize remote connections in a hierarchical
catalog, manage them with a CLI or keyboard-driven TUI, and open SSH sessions and session-only TCP
tunnels with host-key verification. Orza keeps its catalog separate from your OpenSSH configuration.

## Install

Download the archive for your system from the [latest release](https://github.com/pluque01/orza/releases/latest).
Linux and macOS archives use `.tar.gz`. Windows archives use `.zip`. Choose `amd64` for x86-64
systems or `arm64` for ARM64 systems. macOS archives are named `darwin`.

Use the [installation guide](docs/user-guide.md#requirements-and-downloads) to verify the archive's
SHA-256 checksum. Extract the archive and put `orza` (or `orza.exe` on Windows) in a directory on
your `PATH`. Check the installation with `orza --version`, then run `orza` in a VT-capable terminal
to open the TUI.

## Documentation

The [user guide](docs/user-guide.md) covers source builds, CLI and TUI usage, authentication, host
verification, and data recovery. Orza is in pre-1.0 development.

See [contributing](CONTRIBUTING.md) and [security reporting](SECURITY.md).
