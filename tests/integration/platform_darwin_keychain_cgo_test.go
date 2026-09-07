//go:build darwin && cgo

package integration

import (
	"encoding/hex"
	"fmt"

	keychain "github.com/keybase/go-keychain"
	"github.com/pluque01/orza/internal/credential"
)

func darwinKeychainTestUnavailable() string { return "" }

func verifyDarwinKeychainNotSynchronizing(key credential.Key) error {
	query := keychain.NewItem()
	query.SetSecClass(keychain.SecClassGenericPassword)
	query.SetService("orza/" + hex.EncodeToString([]byte(key.Scope)))
	query.SetAccount(hex.EncodeToString([]byte(key.Reference)))
	query.SetSynchronizable(keychain.SynchronizableNo)
	query.SetMatchLimit(keychain.MatchLimitOne)
	query.SetReturnData(true)
	results, err := keychain.QueryItem(query)
	if err != nil {
		return err
	}
	for i := range results {
		clear(results[i].Data)
	}
	if len(results) != 1 {
		return fmt.Errorf("non-synchronizing query returned %d items, want 1", len(results))
	}

	query.SetSynchronizable(keychain.SynchronizableYes)
	results, err = keychain.QueryItem(query)
	if err != nil {
		return err
	}
	for i := range results {
		clear(results[i].Data)
	}
	if len(results) != 0 {
		return fmt.Errorf("synchronizing query matched %d disposable items, want 0", len(results))
	}
	return nil
}
