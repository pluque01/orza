//go:build windows

package credential

import (
	"bytes"
	"context"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/danieljoos/wincred"
)

const windowsCredentialUserName = "orza"

// Store uses generic, current-user Windows Credential Manager entries.
type Store struct{}

func NewStore() *Store { return &Store{} }

func (s *Store) Set(ctx context.Context, key Key, secret []byte) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entry := wincred.NewGenericCredential(windowsTarget(key))
	entry.Persist = wincred.PersistLocalMachine
	entry.UserName = windowsCredentialUserName
	entry.CredentialBlob = append([]byte(nil), secret...)
	defer wipe(entry.CredentialBlob)
	if err := entry.Write(); err != nil {
		return fmt.Errorf("%w: write Windows credential: %v", ErrUnavailable, err)
	}
	actual, err := s.Get(context.WithoutCancel(ctx), key)
	if err != nil {
		return fmt.Errorf("verify Windows credential: %w", err)
	}
	defer wipe(actual)
	if !bytes.Equal(actual, secret) {
		return fmt.Errorf("%w: Windows credential value mismatch", ErrUnavailable)
	}
	return ctx.Err()
}

func (s *Store) Get(ctx context.Context, key Key) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	entry, err := wincred.GetGenericCredential(windowsTarget(key))
	if errors.Is(err, wincred.ErrElementNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("%w: read Windows credential: %v", ErrUnavailable, err)
	}
	secret := append([]byte(nil), entry.CredentialBlob...)
	wipe(entry.CredentialBlob)
	return secret, nil
}

func (s *Store) Delete(ctx context.Context, key Key) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	entry, err := wincred.GetGenericCredential(windowsTarget(key))
	if errors.Is(err, wincred.ErrElementNotFound) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("%w: read Windows credential for delete: %v", ErrUnavailable, err)
	}
	defer wipe(entry.CredentialBlob)
	if err := entry.Delete(); err != nil && !errors.Is(err, wincred.ErrElementNotFound) {
		return fmt.Errorf("%w: delete Windows credential: %v", ErrUnavailable, err)
	}
	_, err = s.Get(context.WithoutCancel(ctx), key)
	if !errors.Is(err, ErrNotFound) {
		if err == nil {
			err = errors.New("credential remains present")
		}
		return fmt.Errorf("%w: verify Windows credential deletion: %v", ErrUnavailable, err)
	}
	return ctx.Err()
}

func windowsTarget(key Key) string {
	return "orza/" + hex.EncodeToString([]byte(key.Scope)) + "/" + hex.EncodeToString([]byte(key.Reference))
}

var _ CredentialStore = (*Store)(nil)
