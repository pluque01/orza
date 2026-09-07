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
type Store struct{}

func NewStore() *Store { return &Store{} }

func (s *Store) Set(ctx context.Context, key Key, secret []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	query := darwinQuery(key)
	update := keychain.NewItem()
	update.SetData(secret)
	err := keychain.UpdateItem(query, update)
	if err == keychain.ErrorItemNotFound {
		item := darwinQuery(key)
		item.SetLabel(darwinCredentialLabel)
		item.SetData(secret)
		item.SetAccessible(keychain.AccessibleWhenUnlocked)
		err = keychain.AddItem(item)
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
	query := darwinQuery(key)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)
	results, err := keychain.QueryItem(query)
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
	err := keychain.DeleteItem(darwinQuery(key))
	if err != nil && err != keychain.ErrorItemNotFound {
		return darwinError(ctx, "delete", err)
	}
	_, err = s.Get(context.WithoutCancel(ctx), key)
	if !errors.Is(err, ErrNotFound) {
		if err == nil {
			err = errors.New("credential remains present")
		}
		return fmt.Errorf("%w: verify macOS Keychain credential deletion: %v", ErrUnavailable, err)
	}
	return ctx.Err()
}

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
