package credential

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
)

func TestFakeScopesAndCopiesSecrets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := NewFake()
	first := Key{Scope: "catalog-a", Reference: "same-ref"}
	second := Key{Scope: "catalog-b", Reference: "same-ref"}
	secret := []byte("first-secret")

	if err := fake.Set(ctx, first, secret); err != nil {
		t.Fatal(err)
	}
	secret[0] = 'X'
	if err := fake.Set(ctx, second, []byte("second-secret")); err != nil {
		t.Fatal(err)
	}

	got, err := fake.Get(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "first-secret" {
		t.Fatalf("got %q", got)
	}
	got[0] = 'X'
	again, err := fake.Get(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	if string(again) != "first-secret" {
		t.Fatalf("Get returned shared storage: %q", again)
	}
}

func TestFakeFaultsArePerOperation(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	fake := NewFake()
	key := Key{Scope: "catalog", Reference: "credential"}
	want := errors.New("locked")
	fake.SetFault(OperationGet, want)

	if err := fake.Set(ctx, key, []byte("secret")); err != nil {
		t.Fatalf("Set affected by Get fault: %v", err)
	}
	if _, err := fake.Get(ctx, key); !errors.Is(err, want) {
		t.Fatalf("Get error = %v, want %v", err, want)
	}
	if err := fake.Delete(ctx, key); err != nil {
		t.Fatalf("Delete affected by Get fault: %v", err)
	}
}

func TestFakeFormattingRedactsSecrets(t *testing.T) {
	t.Parallel()
	fake := NewFake()
	secret := "format-canary"
	if err := fake.Set(context.Background(), Key{Scope: "scope", Reference: "ref"}, []byte(secret)); err != nil {
		t.Fatal(err)
	}

	for _, formatted := range []string{fmt.Sprint(fake), fmt.Sprintf("%+v", fake), fmt.Sprintf("%#v", fake)} {
		if strings.Contains(formatted, secret) {
			t.Fatalf("formatted fake exposed secret: %s", formatted)
		}
	}
}

func TestFakeConcurrentAccess(t *testing.T) {
	t.Parallel()
	fake := NewFake()
	ctx := context.Background()
	var group sync.WaitGroup
	for i := range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			key := Key{Scope: "scope", Reference: Reference(fmt.Sprint(i))}
			if err := fake.Set(ctx, key, []byte("secret")); err != nil {
				t.Errorf("Set: %v", err)
				return
			}
			if _, err := fake.Get(ctx, key); err != nil {
				t.Errorf("Get: %v", err)
			}
		}()
	}
	group.Wait()
}
