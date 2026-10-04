//go:build !linux && !darwin && !windows

package credential

import (
	"context"
	"errors"
	"testing"
)

func TestUnsupportedStoreOptions(t *testing.T) {
	for _, noUI := range []bool{false, true} {
		store := NewStoreWithOptions(StoreOptions{NonInteractive: noUI})
		if _, err := store.Get(context.Background(), Key{}); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
		if err := store.Set(context.Background(), Key{}, nil); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
		if err := store.Delete(context.Background(), Key{}); !errors.Is(err, ErrUnavailable) {
			t.Fatal(err)
		}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := store.Get(ctx, Key{}); !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	}
}
