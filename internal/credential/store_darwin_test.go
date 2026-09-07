//go:build darwin && cgo

package credential

import "testing"

func TestDarwinCredentialBranding(t *testing.T) {
	if darwinCredentialLabel != "Orza credential" {
		t.Fatalf("credential label = %q, want Orza credential", darwinCredentialLabel)
	}
	if got, want := darwinService("catalog"), "orza/636174616c6f67"; got != want {
		t.Fatalf("darwinService() = %q, want %q", got, want)
	}
}
