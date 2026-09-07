//go:build darwin && !cgo

package integration

import "github.com/pluque01/orza/internal/credential"

func darwinKeychainTestUnavailable() string {
	return "macOS Keychain native integration requires cgo and Security.framework"
}

func verifyDarwinKeychainNotSynchronizing(credential.Key) error { return nil }
