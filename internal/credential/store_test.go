package credential

import (
	"context"
	"testing"
)

func TestInteractionSelection(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	defer cancel()
	ctx := WithoutInteraction(parent)
	if IsNonInteractive(parent) || !IsNonInteractive(ctx) || !IsNonInteractive(context.WithoutCancel(ctx)) {
		t.Fatal("interaction selection must be scoped and survive recovery verification contexts")
	}
	cancel()
	if ctx.Err() != context.Canceled {
		t.Fatal("selection must preserve cancellation")
	}
	options := StoreOptions{NonInteractive: true}
	store := NewStoreWithOptions(options)
	options.NonInteractive = false
	if store == nil || NewStore() == nil {
		t.Fatal("constructors must remain lazy and return stores")
	}
}
