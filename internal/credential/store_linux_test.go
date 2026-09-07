//go:build linux

package credential

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"

	dbus "github.com/keybase/dbus"
	secretservice "github.com/keybase/go-keychain/secretservice"
)

func TestLinuxCredentialBranding(t *testing.T) {
	if linuxApplicationAttribute != "orza.application" || linuxScopeAttribute != "orza.scope" ||
		linuxReferenceAttribute != "orza.reference" || linuxCredentialLabel != "Orza credential" {
		t.Fatalf("credential branding = %q/%q/%q/%q", linuxApplicationAttribute, linuxScopeAttribute, linuxReferenceAttribute, linuxCredentialLabel)
	}
	attributes := linuxAttributes(Key{Scope: "catalog", Reference: "password"})
	if attributes["orza.application"] != "orza" || attributes["orza.scope"] != "catalog" || attributes["orza.reference"] != "password" {
		t.Fatalf("credential attributes = %#v", attributes)
	}
}

func TestLinuxStoreCRUDReplacementAndExactKeys(t *testing.T) {
	backend := newFakeLinuxBackend()
	store := newLinuxStore(backend)
	primary := Key{Scope: "catalog-a", Reference: "credential"}
	otherScope := Key{Scope: "catalog-b", Reference: "credential"}
	otherReference := Key{Scope: "catalog-a", Reference: "other"}

	original := []byte("old")
	if err := store.Set(context.Background(), primary, original); err != nil {
		t.Fatalf("Set(primary) error = %v", err)
	}
	original[0] = 'X'
	if err := store.Set(context.Background(), otherScope, []byte("scope")); err != nil {
		t.Fatalf("Set(other scope) error = %v", err)
	}
	if err := store.Set(context.Background(), otherReference, []byte("reference")); err != nil {
		t.Fatalf("Set(other reference) error = %v", err)
	}
	if err := store.Set(context.Background(), primary, []byte("new")); err != nil {
		t.Fatalf("replacement Set(primary) error = %v", err)
	}

	assertLinuxSecret(t, store, primary, "new")
	assertLinuxSecret(t, store, otherScope, "scope")
	assertLinuxSecret(t, store, otherReference, "reference")
	if got := backend.matchingItems(linuxAttributes(primary)); got != 1 {
		t.Fatalf("matching primary items = %d, want 1 after replacement", got)
	}

	if err := store.Delete(context.Background(), primary); err != nil {
		t.Fatalf("Delete(primary) error = %v", err)
	}
	if _, err := store.Get(context.Background(), primary); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get(deleted primary) error = %v, want ErrNotFound", err)
	}
	if err := store.Delete(context.Background(), primary); err != nil {
		t.Fatalf("Delete(missing primary) error = %v", err)
	}
	assertLinuxSecret(t, store, otherScope, "scope")
	assertLinuxSecret(t, store, otherReference, "reference")

	for _, query := range backend.queries() {
		if len(query) != 3 || query[linuxApplicationAttribute] != "orza" ||
			query[linuxScopeAttribute] == "" || query[linuxReferenceAttribute] == "" {
			t.Fatalf("Search() attributes = %#v, want exact application/scope/reference attributes", query)
		}
	}
}

func TestLinuxStoreNotFound(t *testing.T) {
	store := newLinuxStore(newFakeLinuxBackend())
	key := Key{Scope: "scope", Reference: "missing"}
	if _, err := store.Get(context.Background(), key); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Get() error = %v, want ErrNotFound", err)
	}
	if err := store.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete() error = %v, want nil", err)
	}
}

func TestLinuxSecretServiceMissingDefaultCollectionIsEmpty(t *testing.T) {
	for _, test := range []struct {
		name       string
		collection dbus.ObjectPath
		want       bool
	}{
		{name: "missing", collection: dbus.ObjectPath(secretservice.NullPrompt)},
		{name: "existing", collection: "/org/freedesktop/secrets/collection/login", want: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := defaultCollectionExists(test.collection); got != test.want {
				t.Fatalf("collection existence = %v, want %v", got, test.want)
			}
		})
	}
}

func TestLinuxStoreVerificationFailures(t *testing.T) {
	t.Run("write value", func(t *testing.T) {
		backend := newFakeLinuxBackend()
		backend.corruptCreate = true
		err := newLinuxStore(backend).Set(context.Background(), Key{Scope: "scope", Reference: "key"}, []byte("secret"))
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("Set() error = %v, want ErrUnavailable", err)
		}
	})

	t.Run("delete absence", func(t *testing.T) {
		backend := newFakeLinuxBackend()
		store := newLinuxStore(backend)
		key := Key{Scope: "scope", Reference: "key"}
		if err := store.Set(context.Background(), key, []byte("secret")); err != nil {
			t.Fatalf("setup Set() error = %v", err)
		}
		backend.retainDelete = true
		if err := store.Delete(context.Background(), key); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("Delete() error = %v, want ErrUnavailable", err)
		}
	})

	t.Run("ambiguous match", func(t *testing.T) {
		backend := newFakeLinuxBackend()
		attributes := linuxAttributes(Key{Scope: "scope", Reference: "key"})
		backend.add(attributes, []byte("one"))
		backend.add(attributes, []byte("two"))
		_, err := newLinuxStore(backend).Get(context.Background(), Key{Scope: "scope", Reference: "key"})
		if !errors.Is(err, ErrUnavailable) {
			t.Fatalf("Get() error = %v, want ErrUnavailable", err)
		}
	})
}

func TestLinuxStoreBackendErrorsFailClosed(t *testing.T) {
	tests := []struct {
		name  string
		fault func(*fakeLinuxBackend)
		run   func(*Store) error
	}{
		{name: "missing service", fault: func(b *fakeLinuxBackend) { b.openErr = errors.New("service missing") }, run: linuxTestSet},
		{name: "locked collection", fault: func(b *fakeLinuxBackend) { b.unlockErr = errors.New("collection locked") }, run: linuxTestSet},
		{name: "search", fault: func(b *fakeLinuxBackend) { b.searchErr = errors.New("search failed") }, run: linuxTestGet},
		{name: "attributes", fault: func(b *fakeLinuxBackend) { b.attributesErr = errors.New("attributes failed") }, run: linuxTestGetExisting},
		{name: "create", fault: func(b *fakeLinuxBackend) { b.createErr = errors.New("create failed") }, run: linuxTestSet},
		{name: "read", fault: func(b *fakeLinuxBackend) { b.readErr = errors.New("read failed") }, run: linuxTestGetExisting},
		{name: "delete", fault: func(b *fakeLinuxBackend) { b.deleteErr = errors.New("delete failed") }, run: linuxTestDeleteExisting},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			backend := newFakeLinuxBackend()
			test.fault(backend)
			if err := test.run(newLinuxStore(backend)); !errors.Is(err, ErrUnavailable) {
				t.Fatalf("operation error = %v, want ErrUnavailable", err)
			}
		})
	}
}

func TestLinuxStoreDismissedPromptFailsClosed(t *testing.T) {
	for _, test := range []struct {
		name  string
		fault func(*fakeLinuxBackend)
		run   func(*Store) error
	}{
		{name: "unlock", fault: func(b *fakeLinuxBackend) { b.unlockErr = secretservice.PromptDismissedError{} }, run: linuxTestDeleteExisting},
		{name: "create", fault: func(b *fakeLinuxBackend) { b.createErr = secretservice.PromptDismissedError{} }, run: linuxTestSet},
		{name: "delete", fault: func(b *fakeLinuxBackend) { b.deleteErr = secretservice.PromptDismissedError{} }, run: linuxTestDeleteExisting},
	} {
		t.Run(test.name, func(t *testing.T) {
			backend := newFakeLinuxBackend()
			test.fault(backend)
			err := test.run(newLinuxStore(backend))
			if !errors.Is(err, ErrUnavailable) {
				t.Fatalf("operation error = %v, want ErrUnavailable", err)
			}
		})
	}
}

func TestLinuxStoreCancellation(t *testing.T) {
	t.Run("before operation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := newLinuxStore(newFakeLinuxBackend()).Set(ctx, Key{}, nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("Set() error = %v, want context.Canceled", err)
		}
	})

	t.Run("while waiting for serialized operation", func(t *testing.T) {
		backend := newFakeLinuxBackend()
		backend.createStarted = make(chan struct{})
		backend.createRelease = make(chan struct{})
		store := newLinuxStore(backend)
		firstDone := make(chan error, 1)
		go func() {
			firstDone <- store.Set(context.Background(), Key{Scope: "scope", Reference: "first"}, []byte("secret"))
		}()
		<-backend.createStarted

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		if _, err := store.Get(ctx, Key{Scope: "scope", Reference: "second"}); !errors.Is(err, context.Canceled) {
			t.Fatalf("waiting Get() error = %v, want context.Canceled", err)
		}
		close(backend.createRelease)
		if err := <-firstDone; err != nil {
			t.Fatalf("first Set() error = %v", err)
		}
	})

	t.Run("during backend wait", func(t *testing.T) {
		backend := newFakeLinuxBackend()
		backend.block = linuxBackendUnlock
		backend.blockStarted = make(chan struct{})
		backend.blockCanceled = make(chan struct{})
		backend.blockRelease = make(chan struct{})
		backend.unlockErr = errors.New("prompt ended")
		store := newLinuxStore(backend)
		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		go func() {
			done <- store.Set(ctx, Key{Scope: "scope", Reference: "key"}, []byte("secret"))
		}()
		<-backend.blockStarted
		cancel()
		<-backend.blockCanceled
		select {
		case err := <-done:
			t.Fatalf("Set() returned before backend joined: %v", err)
		default:
		}
		close(backend.blockRelease)
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("Set() error = %v, want context.Canceled", err)
		}
	})
}

func TestLinuxStoreCancelsAndJoinsEveryBackendOperation(t *testing.T) {
	key := Key{Scope: "scope", Reference: "key"}
	tests := []struct {
		operation linuxBackendOperation
		prepare   func(*fakeLinuxBackend)
		run       func(context.Context, *Store) error
		assert    func(*testing.T, *fakeLinuxBackend)
	}{
		{operation: linuxBackendOpenSession, run: func(ctx context.Context, store *Store) error {
			return store.Set(ctx, key, []byte("secret"))
		}},
		{operation: linuxBackendUnlock, run: func(ctx context.Context, store *Store) error {
			return store.Set(ctx, key, []byte("secret"))
		}},
		{operation: linuxBackendSearch, run: func(ctx context.Context, store *Store) error {
			_, err := store.Get(ctx, key)
			return err
		}},
		{operation: linuxBackendCreate, run: func(ctx context.Context, store *Store) error {
			return store.Set(ctx, key, []byte("secret"))
		}, assert: func(t *testing.T, backend *fakeLinuxBackend) {
			if got := backend.matchingItems(linuxAttributes(key)); got != 0 {
				t.Fatalf("matching items after canceled create = %d, want 0", got)
			}
		}},
		{operation: linuxBackendRead, prepare: func(backend *fakeLinuxBackend) {
			backend.add(linuxAttributes(key), []byte("secret"))
		}, run: func(ctx context.Context, store *Store) error {
			_, err := store.Get(ctx, key)
			return err
		}},
		{operation: linuxBackendDelete, prepare: func(backend *fakeLinuxBackend) {
			backend.add(linuxAttributes(key), []byte("secret"))
		}, run: func(ctx context.Context, store *Store) error {
			return store.Delete(ctx, key)
		}, assert: func(t *testing.T, backend *fakeLinuxBackend) {
			if got := backend.matchingItems(linuxAttributes(key)); got != 1 {
				t.Fatalf("matching items after canceled delete = %d, want 1", got)
			}
		}},
	}

	for _, test := range tests {
		t.Run(string(test.operation), func(t *testing.T) {
			backend := newFakeLinuxBackend()
			backend.block = test.operation
			backend.blockStarted = make(chan struct{})
			backend.blockCanceled = make(chan struct{})
			backend.blockRelease = make(chan struct{})
			if test.prepare != nil {
				test.prepare(backend)
			}
			ctx, cancel := context.WithCancel(context.Background())
			done := make(chan error, 1)
			go func() { done <- test.run(ctx, newLinuxStore(backend)) }()

			<-backend.blockStarted
			cancel()
			<-backend.blockCanceled
			select {
			case err := <-done:
				t.Fatalf("operation returned before backend joined: %v", err)
			default:
			}
			close(backend.blockRelease)
			if err := <-done; !errors.Is(err, context.Canceled) {
				t.Fatalf("operation error = %v, want context.Canceled", err)
			}
			if test.assert != nil {
				test.assert(t, backend)
			}
		})
	}
}

func assertLinuxSecret(t *testing.T, store *Store, key Key, want string) {
	t.Helper()
	got, err := store.Get(context.Background(), key)
	if err != nil {
		t.Fatalf("Get(%#v) error = %v", key, err)
	}
	defer wipe(got)
	if string(got) != want {
		t.Fatalf("Get(%#v) = %q, want %q", key, got, want)
	}
}

func linuxTestSet(store *Store) error {
	return store.Set(context.Background(), Key{Scope: "scope", Reference: "key"}, []byte("secret"))
}

func linuxTestGet(store *Store) error {
	_, err := store.Get(context.Background(), Key{Scope: "scope", Reference: "key"})
	return err
}

func linuxTestGetExisting(store *Store) error {
	backend := store.backend.(*fakeLinuxBackend)
	backend.add(linuxAttributes(Key{Scope: "scope", Reference: "key"}), []byte("secret"))
	return linuxTestGet(store)
}

func linuxTestDeleteExisting(store *Store) error {
	backend := store.backend.(*fakeLinuxBackend)
	backend.add(linuxAttributes(Key{Scope: "scope", Reference: "key"}), []byte("secret"))
	return store.Delete(context.Background(), Key{Scope: "scope", Reference: "key"})
}

type fakeLinuxItem struct {
	attributes map[string]string
	secret     []byte
}

type linuxBackendOperation string

const (
	linuxBackendOpenSession linuxBackendOperation = "open session"
	linuxBackendUnlock      linuxBackendOperation = "unlock"
	linuxBackendSearch      linuxBackendOperation = "search"
	linuxBackendCreate      linuxBackendOperation = "create"
	linuxBackendRead        linuxBackendOperation = "read"
	linuxBackendDelete      linuxBackendOperation = "delete"
)

type fakeLinuxBackend struct {
	mu sync.Mutex

	items map[string]fakeLinuxItem
	next  int
	query []map[string]string

	openErr       error
	unlockErr     error
	searchErr     error
	attributesErr error
	createErr     error
	readErr       error
	deleteErr     error

	corruptCreate bool
	retainDelete  bool
	createStarted chan struct{}
	createRelease chan struct{}

	block         linuxBackendOperation
	blockStarted  chan struct{}
	blockCanceled chan struct{}
	blockRelease  chan struct{}
}

func newFakeLinuxBackend() *fakeLinuxBackend {
	return &fakeLinuxBackend{items: make(map[string]fakeLinuxItem)}
}

func (b *fakeLinuxBackend) OpenSession(ctx context.Context) (*linuxSession, error) {
	if err := b.wait(ctx, linuxBackendOpenSession); err != nil {
		return nil, err
	}
	if b.openErr != nil {
		return nil, b.openErr
	}
	return &linuxSession{}, nil
}

func (b *fakeLinuxBackend) CloseSession(*linuxSession) {}

func (b *fakeLinuxBackend) Unlock(ctx context.Context) error {
	if err := b.wait(ctx, linuxBackendUnlock); err != nil {
		return err
	}
	return b.unlockErr
}

func (b *fakeLinuxBackend) Search(ctx context.Context, attributes map[string]string) ([]string, error) {
	if err := b.wait(ctx, linuxBackendSearch); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.query = append(b.query, cloneLinuxAttributes(attributes))
	if b.searchErr != nil {
		return nil, b.searchErr
	}
	var matches []string
	for id, item := range b.items {
		if linuxTestAttributesMatch(item.attributes, attributes) {
			matches = append(matches, id)
		}
	}
	return matches, nil
}

func (b *fakeLinuxBackend) Attributes(ctx context.Context, item string) (map[string]string, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.attributesErr != nil {
		return nil, b.attributesErr
	}
	entry, ok := b.items[item]
	if !ok {
		return nil, errors.New("item missing")
	}
	return cloneLinuxAttributes(entry.attributes), nil
}

func (b *fakeLinuxBackend) Create(ctx context.Context, _ *linuxSession, attributes map[string]string, secret []byte, replace bool) error {
	if b.createStarted != nil {
		close(b.createStarted)
		<-b.createRelease
	}
	if err := b.wait(ctx, linuxBackendCreate); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.createErr != nil {
		return b.createErr
	}
	if replace {
		for id, item := range b.items {
			if linuxTestAttributesMatch(item.attributes, attributes) {
				wipe(item.secret)
				delete(b.items, id)
			}
		}
	}
	value := clone(secret)
	if b.corruptCreate {
		wipe(value)
		value = []byte("corrupt")
	}
	b.addLocked(attributes, value)
	return nil
}

func (b *fakeLinuxBackend) Read(ctx context.Context, _ *linuxSession, item string) ([]byte, error) {
	if err := b.wait(ctx, linuxBackendRead); err != nil {
		return nil, err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.readErr != nil {
		return nil, b.readErr
	}
	entry, ok := b.items[item]
	if !ok {
		return nil, errors.New("item missing")
	}
	return clone(entry.secret), nil
}

func (b *fakeLinuxBackend) Delete(ctx context.Context, item string) error {
	if err := b.wait(ctx, linuxBackendDelete); err != nil {
		return err
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.deleteErr != nil {
		return b.deleteErr
	}
	if b.retainDelete {
		return nil
	}
	entry := b.items[item]
	wipe(entry.secret)
	delete(b.items, item)
	return nil
}

func (b *fakeLinuxBackend) wait(ctx context.Context, operation linuxBackendOperation) error {
	if b.block != operation {
		return ctx.Err()
	}
	close(b.blockStarted)
	<-ctx.Done()
	close(b.blockCanceled)
	<-b.blockRelease
	return ctx.Err()
}

func (b *fakeLinuxBackend) add(attributes map[string]string, secret []byte) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.addLocked(attributes, clone(secret))
}

func (b *fakeLinuxBackend) addLocked(attributes map[string]string, secret []byte) {
	b.next++
	b.items[fmt.Sprintf("item-%d", b.next)] = fakeLinuxItem{
		attributes: cloneLinuxAttributes(attributes),
		secret:     secret,
	}
}

func (b *fakeLinuxBackend) matchingItems(attributes map[string]string) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	count := 0
	for _, item := range b.items {
		if linuxTestAttributesMatch(item.attributes, attributes) {
			count++
		}
	}
	return count
}

func (b *fakeLinuxBackend) queries() []map[string]string {
	b.mu.Lock()
	defer b.mu.Unlock()
	queries := make([]map[string]string, len(b.query))
	for i, query := range b.query {
		queries[i] = cloneLinuxAttributes(query)
	}
	return queries
}

func linuxTestAttributesMatch(attributes, query map[string]string) bool {
	for name, value := range query {
		if attributes[name] != value {
			return false
		}
	}
	return true
}

func cloneLinuxAttributes(attributes map[string]string) map[string]string {
	result := make(map[string]string, len(attributes))
	for name, value := range attributes {
		result[name] = value
	}
	return result
}
