//go:build darwin && !cgo

package credential

import (
	"context"
	"fmt"
)

// Store is unavailable without cgo because Security.framework is a C API.
type Store struct{}

func NewStore() *Store { return &Store{} }

func (s *Store) Set(ctx context.Context, _ Key, _ []byte) error { return darwinUnavailable(ctx) }

func (s *Store) Get(ctx context.Context, _ Key) ([]byte, error) { return nil, darwinUnavailable(ctx) }

func (s *Store) Delete(ctx context.Context, _ Key) error { return darwinUnavailable(ctx) }

func darwinUnavailable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return fmt.Errorf("%w: macOS Keychain requires cgo", ErrUnavailable)
}

var _ CredentialStore = (*Store)(nil)
