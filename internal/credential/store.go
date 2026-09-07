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
