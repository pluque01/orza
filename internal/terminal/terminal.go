// Package terminal defines the local terminal boundary used by interactive sessions.
package terminal

import (
	"context"
	"errors"
)

// ErrNoSecretResponse is returned by deterministic implementations when no
// response has been configured for a secret prompt.
var ErrNoSecretResponse = errors.New("no secret response available")

// ErrSecretQuit distinguishes Ctrl+C from Esc at the terminal-owned secret
// editor so callers can preserve deferred quit semantics.
var ErrSecretQuit = errors.New("secret input requested quit")

// ErrRequiredVT is returned when a terminal is confirmed to lack VT input or
// output support and the adapter has no safe fallback.
var ErrRequiredVT = errors.New("required VT capabilities are unavailable; use a VT-capable terminal with VT input and output enabled, then retry")

// VTStatus describes only capabilities that can be established reliably.
type VTStatus uint8

const (
	// VTUnverified means the platform has no reliable runtime capability query.
	VTUnverified VTStatus = iota
	VTSupported
	VTMissing
)

// VTCapability reports required VT support and whether the terminal adapter
// can safely compensate when that support is missing.
type VTCapability struct {
	Status       VTStatus
	SafeFallback bool
}

// Size is a terminal or remote PTY size, measured in character cells.
type Size struct {
	Columns int
	Rows    int
}

// Valid reports whether both dimensions are positive.
func (s Size) Valid() bool {
	return s.Columns > 0 && s.Rows > 0
}

// State is an opaque terminal snapshot suitable for a later Restore call.
// Platform implementations in this package attach their native state to it.
type State struct {
	raw        bool
	generation uint64
	native     any
}

// Raw reports whether the captured state used raw input mode.
func (s State) Raw() bool {
	return s.raw
}

// SecretPrompt contains only display metadata; the response is never retained
// in the prompt or in terminal call records.
type SecretPrompt struct {
	Message string
}

// Terminal owns local terminal state and no-echo input. ResizeEvents returns a
// stream which is closed when its context is canceled or watching stops.
type Terminal interface {
	Interactive() bool
	Capture(context.Context) (State, error)
	VTCapability(context.Context) (VTCapability, error)
	Size(context.Context) (Size, error)
	MakeRaw(context.Context) error
	Restore(context.Context, State) error
	ReadSecret(context.Context, SecretPrompt) ([]byte, error)
	ResizeEvents(context.Context) (<-chan Size, error)
}
