//go:build darwin && cgo

package credential

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	keychain "github.com/keybase/go-keychain"
)

const darwinCredentialLabel = "Orza credential"

// Store uses non-synchronizing generic-password entries in the login Keychain.
type Store struct {
	nonInteractive bool
	backend        darwinBackend
}

func NewStore() *Store { return &Store{backend: nativeDarwinBackend{}} }

// NewStoreWithOptions constructs a lazy native store with immutable UI policy.
func NewStoreWithOptions(options StoreOptions) CredentialStore {
	return &Store{nonInteractive: options.NonInteractive, backend: nativeDarwinBackend{}}
}

func (s *Store) Set(ctx context.Context, key Key, secret []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	query := s.query(key)
	update := keychain.NewItem()
	update.SetData(secret)
	err := s.backend.Update(query, update)
	if err == keychain.ErrorItemNotFound {
		item := s.query(key)
		item.SetLabel(darwinCredentialLabel)
		item.SetData(secret)
		item.SetAccessible(keychain.AccessibleWhenUnlocked)
		err = s.backend.Add(item)
	}
	if err != nil {
		return darwinError(ctx, "write", err)
	}
	actual, err := s.Get(context.WithoutCancel(ctx), key)
	if err != nil {
		return fmt.Errorf("verify macOS Keychain credential: %w", err)
	}
	defer wipe(actual)
	if !bytes.Equal(actual, secret) {
		return fmt.Errorf("%w: macOS Keychain credential value mismatch", ErrUnavailable)
	}
	return ctx.Err()
}

func (s *Store) Get(ctx context.Context, key Key) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	query := s.query(key)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)
	results, err := s.backend.Query(query)
	if err != nil {
		return nil, darwinError(ctx, "read", err)
	}
	if len(results) == 0 {
		return nil, ErrNotFound
	}
	if len(results) != 1 {
		return nil, fmt.Errorf("%w: macOS Keychain returned multiple credentials", ErrUnavailable)
	}
	secret := append([]byte(nil), results[0].Data...)
	wipe(results[0].Data)
	return secret, nil
}

func (s *Store) Delete(ctx context.Context, key Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	err := s.backend.Delete(s.query(key))
	if err != nil && err != keychain.ErrorItemNotFound {
		return darwinError(ctx, "delete", err)
	}
	remaining, err := s.Get(context.WithoutCancel(ctx), key)
	wipe(remaining)
	if !errors.Is(err, ErrNotFound) {
		if err == nil {
			err = errors.New("credential remains present")
		}
		return fmt.Errorf("%w: verify macOS Keychain credential deletion: %v", ErrUnavailable, err)
	}
	return ctx.Err()
}

func (s *Store) query(key Key) keychain.Item {
	item := darwinQuery(key)
	if s.nonInteractive {
		name, value := darwinAuthenticationUI()
		item.SetString(name, value)
	}
	return item
}

type darwinBackend interface {
	Query(keychain.Item) ([]keychain.QueryResult, error)
	Update(keychain.Item, keychain.Item) error
	Add(keychain.Item) error
	Delete(keychain.Item) error
}

type nativeDarwinBackend struct{}

func (nativeDarwinBackend) Query(item keychain.Item) ([]keychain.QueryResult, error) {
	return keychain.QueryItem(item)
}
func (nativeDarwinBackend) Update(query, update keychain.Item) error {
	return keychain.UpdateItem(query, update)
}
func (nativeDarwinBackend) Add(item keychain.Item) error    { return keychain.AddItem(item) }
func (nativeDarwinBackend) Delete(item keychain.Item) error { return keychain.DeleteItem(item) }

func darwinQuery(key Key) keychain.Item {
	item := keychain.NewItem()
	item.SetSecClass(keychain.SecClassGenericPassword)
	item.SetService(darwinService(key.Scope))
	item.SetAccount(hex.EncodeToString([]byte(key.Reference)))
	item.SetSynchronizable(keychain.SynchronizableNo)
	return item
}

func darwinService(scope Scope) string {
	return "orza/" + hex.EncodeToString([]byte(scope))
}

func darwinError(ctx context.Context, operation string, err error) error {
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if err == keychain.ErrorUserCanceled {
		return fmt.Errorf("%w: macOS Keychain %s canceled", ErrUnavailable, operation)
	}
	return fmt.Errorf("%w: macOS Keychain %s: %v", ErrUnavailable, operation, err)
}

var _ CredentialStore = (*Store)(nil)
