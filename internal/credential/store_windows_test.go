//go:build windows

package credential

import (
	"context"
	"errors"
	"testing"

	"github.com/danieljoos/wincred"
)

func TestWindowsCredentialBranding(t *testing.T) {
	if windowsCredentialUserName != "orza" {
		t.Fatalf("credential user name = %q, want orza", windowsCredentialUserName)
	}
	if got, want := windowsTarget(Key{Scope: "catalog", Reference: "password"}), "orza/636174616c6f67/70617373776f7264"; got != want {
		t.Fatalf("windowsTarget() = %q, want %q", got, want)
	}
}

func TestWindowsNonInteractiveGenericAPIs(t *testing.T) {
	backend := &fakeWindowsBackend{}
	store := &Store{nonInteractive: true, backend: backend}
	if err := store.Set(context.Background(), Key{}, []byte("secret")); err != nil {
		t.Fatal(err)
	}
	if backend.reads != 1 || backend.writes != 1 {
		t.Fatal("write must verify using generic read")
	}
	if err := store.Delete(context.Background(), Key{}); err != nil {
		t.Fatal(err)
	}
	if backend.deletes != 1 || backend.reads != 3 {
		t.Fatal("delete must verify using generic read")
	}
	if _, err := store.Get(context.Background(), Key{}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing = %v", err)
	}
	if err := store.Delete(context.Background(), Key{}); err != nil {
		t.Fatal(err)
	}
	backend.err = errors.New("access denied")
	if _, err := store.Get(context.Background(), Key{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("denied = %v", err)
	}
	if err := store.Delete(context.Background(), Key{}); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("denied delete = %v", err)
	}
	if err := store.Set(context.Background(), Key{}, nil); !errors.Is(err, ErrUnavailable) {
		t.Fatalf("denied write = %v", err)
	}
	if !NewStoreWithOptions(StoreOptions{NonInteractive: true}).(*Store).nonInteractive || NewStore().nonInteractive {
		t.Fatal("constructor policy")
	}
}

func TestWindowsNonInteractiveVerificationUnavailable(t *testing.T) {
	for _, operation := range []string{"set", "delete"} {
		backend := &fakeWindowsBackend{present: true, denyAfterMutation: true}
		store := &Store{nonInteractive: true, backend: backend}
		var err error
		if operation == "set" {
			err = store.Set(context.Background(), Key{}, []byte("secret"))
		} else {
			err = store.Delete(context.Background(), Key{})
		}
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("%s verification = %v", operation, err)
		}
	}
}

type fakeWindowsBackend struct {
	present, denyAfterMutation bool
	err                        error
	reads, writes, deletes     int
}

func (b *fakeWindowsBackend) Read(target string) (*wincred.GenericCredential, error) {
	b.reads++
	if b.err != nil {
		return nil, b.err
	}
	if !b.present {
		return nil, wincred.ErrElementNotFound
	}
	entry := wincred.NewGenericCredential(target)
	entry.CredentialBlob = []byte("secret")
	return entry, nil
}

func (b *fakeWindowsBackend) Write(_ *wincred.GenericCredential) error {
	b.writes++
	if b.err != nil {
		return b.err
	}
	b.present = true
	if b.denyAfterMutation {
		b.err = errors.New("access denied")
	}
	return nil
}

func (b *fakeWindowsBackend) Delete(_ *wincred.GenericCredential) error {
	b.deletes++
	if b.err != nil {
		return b.err
	}
	b.present = false
	if b.denyAfterMutation {
		b.err = errors.New("access denied")
	}
	return nil
}
