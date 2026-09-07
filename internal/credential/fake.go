package credential

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// Operation identifies a credential-store operation for calls and faults.
type Operation string

const (
	OperationSet    Operation = "set"
	OperationGet    Operation = "get"
	OperationDelete Operation = "delete"
)

// Call records non-secret information about one store operation.
type Call struct {
	Operation Operation
	Key       Key
}

// Fake is an isolated, concurrency-safe in-memory CredentialStore.
// Its zero value is ready for use.
type Fake struct {
	mu      sync.Mutex
	secrets map[Key][]byte
	faults  map[Operation]error
	calls   []Call
}

// NewFake returns an empty in-memory credential store.
func NewFake() *Fake {
	return &Fake{}
}

// SetFault makes every subsequent operation of the given kind return err.
// Passing nil clears the fault.
func (f *Fake) SetFault(operation Operation, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err == nil {
		delete(f.faults, operation)
		return
	}
	if f.faults == nil {
		f.faults = make(map[Operation]error)
	}
	f.faults[operation] = err
}

// Calls returns a snapshot of calls in invocation order.
func (f *Fake) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()

	return append([]Call(nil), f.calls...)
}

// Lookup returns a copy of a stored secret for test assertions.
func (f *Fake) Lookup(key Key) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()

	secret, ok := f.secrets[key]
	return clone(secret), ok
}

// Set stores a private copy of secret.
func (f *Fake) Set(ctx context.Context, key Key, secret []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	f.calls = append(f.calls, Call{Operation: OperationSet, Key: key})
	if err := f.faults[OperationSet]; err != nil {
		return err
	}
	if f.secrets == nil {
		f.secrets = make(map[Key][]byte)
	}
	wipe(f.secrets[key])
	f.secrets[key] = clone(secret)
	return nil
}

// Get returns a private copy of the stored secret.
func (f *Fake) Get(ctx context.Context, key Key) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.calls = append(f.calls, Call{Operation: OperationGet, Key: key})
	if err := f.faults[OperationGet]; err != nil {
		return nil, err
	}
	secret, ok := f.secrets[key]
	if !ok {
		return nil, ErrNotFound
	}
	return clone(secret), nil
}

// Delete removes a secret. Deleting an absent key is idempotent.
func (f *Fake) Delete(ctx context.Context, key Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	f.calls = append(f.calls, Call{Operation: OperationDelete, Key: key})
	if err := f.faults[OperationDelete]; err != nil {
		return err
	}
	wipe(f.secrets[key])
	delete(f.secrets, key)
	return nil
}

// Format prevents fmt from traversing the fake's secret-bearing fields.
func (f *Fake) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "credential.Fake{redacted}")
}

func clone(value []byte) []byte {
	return append([]byte(nil), value...)
}

func wipe(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
