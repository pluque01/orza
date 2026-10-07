// Package credential defines access to an operating system credential store.
package credential

import (
	"context"
	"errors"
)

var (
	// ErrNotFound means that no secret exists for a key.
	ErrNotFound = errors.New("credential not found")
	// ErrUnavailable means that the platform's secure store cannot be used.
	ErrUnavailable = errors.New("credential store unavailable")
)

// Scope separates credentials belonging to different catalogs.
type Scope string

// Reference is an opaque, non-secret identifier stored in the catalog.
type Reference string

// Key identifies one secret without containing secret material.
type Key struct {
	Scope     Scope
	Reference Reference
}

// CredentialStore is the cancellable contract implemented by native credential stores.
// Implementations must not retain or return the caller's byte slice directly.
type CredentialStore interface {
	Set(context.Context, Key, []byte) error
	Get(context.Context, Key) ([]byte, error)
	Delete(context.Context, Key) error
}

// StoreOptions selects native behavior once, before any recovery or secret access.
// The zero value preserves interactive native-store behavior.
// UI suppression does not impose hard deadlines on synchronous native calls.
type StoreOptions struct {
	NonInteractive bool
}

type interactionKey struct{}

// WithoutInteraction carries store selection to bootstrap without changing its
// signature. It does not change an already constructed store's native policy.
func WithoutInteraction(ctx context.Context) context.Context {
	return context.WithValue(ctx, interactionKey{}, true)
}

// IsNonInteractive reports whether bootstrap should select a no-UI store.
func IsNonInteractive(ctx context.Context) bool {
	selected, _ := ctx.Value(interactionKey{}).(bool)
	return selected
}
