//go:build !linux && !darwin && !windows

package credential

import "context"

// Store is unavailable on platforms without a native credential adapter.
type Store struct{}

func NewStore() *Store { return &Store{} }

// NewStoreWithOptions remains unavailable without a native credential adapter.
func NewStoreWithOptions(StoreOptions) CredentialStore { return NewStore() }

func (*Store) Set(ctx context.Context, _ Key, _ []byte) error { return unsupportedUnavailable(ctx) }
func (*Store) Get(ctx context.Context, _ Key) ([]byte, error) {
	return nil, unsupportedUnavailable(ctx)
}
func (*Store) Delete(ctx context.Context, _ Key) error { return unsupportedUnavailable(ctx) }

func unsupportedUnavailable(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrUnavailable
}

var _ CredentialStore = (*Store)(nil)
