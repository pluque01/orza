//go:build darwin && cgo

package credential

import (
	"context"
	"errors"
	"reflect"
	"testing"

	keychain "github.com/keybase/go-keychain"
)

func TestDarwinCredentialBranding(t *testing.T) {
	if darwinCredentialLabel != "Orza credential" {
		t.Fatalf("credential label = %q, want Orza credential", darwinCredentialLabel)
	}
	if got, want := darwinService("catalog"), "orza/636174616c6f67"; got != want {
		t.Fatalf("darwinService() = %q, want %q", got, want)
	}
}

func TestDarwinNonInteractivePolicy(t *testing.T) {
	for _, operation := range []string{"get", "set", "delete"} {
		for _, denied := range []bool{false, true} {
			t.Run(operation+map[bool]string{false: "/allowed", true: "/approval required"}[denied], func(t *testing.T) {
				backend := &fakeDarwinBackend{t: t, present: true, denied: denied}
				store := &Store{nonInteractive: true, backend: backend}
				var err error
				switch operation {
				case "get":
					_, err = store.Get(context.Background(), Key{})
				case "set":
					err = store.Set(context.Background(), Key{}, []byte("secret"))
				case "delete":
					err = store.Delete(context.Background(), Key{})
				}
				if denied && (!errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotFound)) {
					t.Fatalf("denied error = %v", err)
				}
				if !denied && err != nil {
					t.Fatal(err)
				}
			})
		}
	}
	if !NewStoreWithOptions(StoreOptions{NonInteractive: true}).(*Store).nonInteractive || NewStore().nonInteractive {
		t.Fatal("constructor policy")
	}
}

func TestDarwinNonInteractiveVerificationFailsUnavailable(t *testing.T) {
	for _, operation := range []string{"set", "delete"} {
		backend := &fakeDarwinBackend{t: t, present: true, denyAfterMutation: true}
		store := &Store{nonInteractive: true, backend: backend}
		var err error
		if operation == "set" {
			err = store.Set(context.Background(), Key{}, []byte("secret"))
		} else {
			err = store.Delete(context.Background(), Key{})
		}
		if !errors.Is(err, ErrUnavailable) || errors.Is(err, ErrNotFound) {
			t.Fatalf("%s verification = %v", operation, err)
		}
	}
}

type fakeDarwinBackend struct {
	t                                  *testing.T
	present, denied, denyAfterMutation bool
}

func (b *fakeDarwinBackend) check(item keychain.Item) error {
	b.t.Helper()
	name, value := darwinAuthenticationUI()
	attrs := reflect.ValueOf(item).FieldByName("attr")
	got := attrs.MapIndex(reflect.ValueOf(name))
	if !got.IsValid() || got.Elem().String() != value {
		b.t.Fatal("native query/mutation lacks fail-without-UI")
	}
	if b.denied {
		return keychain.Error(-25308)
	} // errSecInteractionNotAllowed
	return nil
}

func (b *fakeDarwinBackend) Query(item keychain.Item) ([]keychain.QueryResult, error) {
	if err := b.check(item); err != nil {
		return nil, err
	}
	if !b.present {
		return nil, nil
	}
	return []keychain.QueryResult{{Data: []byte("secret")}}, nil
}

func (b *fakeDarwinBackend) Update(query, _ keychain.Item) error {
	if err := b.check(query); err != nil {
		return err
	}
	if !b.present {
		return keychain.ErrorItemNotFound
	}
	b.denied = b.denyAfterMutation
	return nil
}

func (b *fakeDarwinBackend) Add(item keychain.Item) error {
	if err := b.check(item); err != nil {
		return err
	}
	b.present = true
	b.denied = b.denyAfterMutation
	return nil
}

func (b *fakeDarwinBackend) Delete(item keychain.Item) error {
	if err := b.check(item); err != nil {
		return err
	}
	b.present = false
	b.denied = b.denyAfterMutation
	return nil
}

func TestDarwinNonInteractiveAddAndMissing(t *testing.T) {
	backend := &fakeDarwinBackend{t: t}
	store := &Store{nonInteractive: true, backend: backend}
	if _, err := store.Get(context.Background(), Key{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
	if err := store.Set(context.Background(), Key{}, []byte("secret")); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), Key{}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(context.Background(), Key{}); err != nil {
		t.Fatal(err)
	}
}
