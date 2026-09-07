//go:build windows

package credential

import "testing"

func TestWindowsCredentialBranding(t *testing.T) {
	if windowsCredentialUserName != "orza" {
		t.Fatalf("credential user name = %q, want orza", windowsCredentialUserName)
	}
	if got, want := windowsTarget(Key{Scope: "catalog", Reference: "password"}), "orza/636174616c6f67/70617373776f7264"; got != want {
		t.Fatalf("windowsTarget() = %q, want %q", got, want)
	}
}
