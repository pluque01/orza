package domain

import (
	"errors"
	"math"
	"strings"
	"testing"
)

func TestNewIDProducesRandomImmutable128BitValues(t *testing.T) {
	t.Parallel()

	seen := make(map[ID]struct{}, 256)
	for range 256 {
		id, err := NewID()
		if err != nil {
			t.Fatalf("NewID() error = %v", err)
		}
		if len(id.String()) != 32 {
			t.Fatalf("NewID().String() length = %d, want 32", len(id.String()))
		}
		if id.IsZero() {
			t.Fatal("NewID() returned the zero ID")
		}
		if _, exists := seen[id]; exists {
			t.Fatalf("NewID() returned duplicate %q", id)
		}
		seen[id] = struct{}{}
	}

	id, err := NewID()
	if err != nil {
		t.Fatalf("NewID() error = %v", err)
	}
	original := id.String()
	bytes := id.Bytes()
	bytes[0] ^= 0xff
	if id.String() != original {
		t.Fatalf("mutating Bytes() result changed ID from %q to %q", original, id)
	}
}

func TestParseIDRequiresCanonicalLowercaseHex(t *testing.T) {
	t.Parallel()

	const canonical = "00112233445566778899aabbccddeeff"
	tests := []struct {
		name  string
		input string
		valid bool
	}{
		{name: "canonical", input: canonical, valid: true},
		{name: "zero", input: strings.Repeat("0", 32), valid: true},
		{name: "uppercase", input: "00112233445566778899AABBCCDDEEFF"},
		{name: "hyphenated", input: "00112233-4455-6677-8899-aabbccddeeff"},
		{name: "short", input: canonical[:31]},
		{name: "long", input: canonical + "0"},
		{name: "non hex", input: "g0112233445566778899aabbccddeeff"},
		{name: "surrounding space", input: " " + canonical},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			id, err := ParseID(tt.input)
			if tt.valid {
				if err != nil {
					t.Fatalf("ParseID(%q) error = %v", tt.input, err)
				}
				if got := id.String(); got != tt.input {
					t.Fatalf("ParseID(%q).String() = %q", tt.input, got)
				}
				text, err := id.MarshalText()
				if err != nil || string(text) != tt.input {
					t.Fatalf("MarshalText() = %q, %v", text, err)
				}
				return
			}

			if !errors.Is(err, ErrInvalidID) {
				t.Fatalf("ParseID(%q) error = %v, want ErrInvalidID", tt.input, err)
			}
			if strings.Contains(err.Error(), tt.input) {
				t.Fatalf("error %q exposes rejected input", err)
			}
		})
	}
}

func TestNodeKinds(t *testing.T) {
	t.Parallel()

	for _, want := range []NodeKind{NodeKindFolder, NodeKindConnection} {
		got, err := ParseNodeKind(want.String())
		if err != nil {
			t.Fatalf("ParseNodeKind(%q) error = %v", want, err)
		}
		if got != want || !got.Valid() {
			t.Fatalf("ParseNodeKind(%q) = %q, valid %t", want, got, got.Valid())
		}
	}

	for _, input := range []string{"", "Folder", "host", "connection "} {
		_, err := ParseNodeKind(input)
		if !errors.Is(err, ErrInvalidNodeKind) {
			t.Errorf("ParseNodeKind(%q) error = %v, want ErrInvalidNodeKind", input, err)
		}
	}
}

func TestRevisionsStartAtOneAndIncrease(t *testing.T) {
	t.Parallel()

	initial := InitialRevision()
	if initial.Uint64() != 1 || !initial.Valid() {
		t.Fatalf("InitialRevision() = %d, valid %t", initial.Uint64(), initial.Valid())
	}

	revision, err := NewRevision(41)
	if err != nil {
		t.Fatalf("NewRevision(41) error = %v", err)
	}
	next, err := revision.Next()
	if err != nil {
		t.Fatalf("Revision.Next() error = %v", err)
	}
	if next.Uint64() != 42 || revision.Uint64() != 41 {
		t.Fatalf("Next() = %d and original = %d, want 42 and 41", next.Uint64(), revision.Uint64())
	}

	if _, err := NewRevision(0); !errors.Is(err, ErrInvalidRevision) {
		t.Fatalf("NewRevision(0) error = %v, want ErrInvalidRevision", err)
	}
	maximum, err := NewRevision(math.MaxUint64)
	if err != nil {
		t.Fatalf("NewRevision(MaxUint64) error = %v", err)
	}
	if _, err := maximum.Next(); !errors.Is(err, ErrRevisionOverflow) {
		t.Fatalf("maximum.Next() error = %v, want ErrRevisionOverflow", err)
	}
}

func FuzzParseID(f *testing.F) {
	f.Add("00112233445566778899aabbccddeeff")
	f.Add("00112233445566778899AABBCCDDEEFF")
	f.Add("")
	f.Fuzz(func(t *testing.T, input string) {
		id, err := ParseID(input)
		if err != nil {
			if !errors.Is(err, ErrInvalidID) {
				t.Fatalf("ParseID(%q) error = %v, want ErrInvalidID", input, err)
			}
			return
		}

		if len(input) != 32 || id.String() != input {
			t.Fatalf("successful ParseID(%q) produced %q", input, id)
		}
		for _, character := range input {
			if !strings.ContainsRune("0123456789abcdef", character) {
				t.Fatalf("ParseID accepted non-canonical input %q", input)
			}
		}
	})
}

func FuzzParseNodeKind(f *testing.F) {
	f.Add("folder")
	f.Add("connection")
	f.Add("Folder")
	f.Add("")
	f.Fuzz(func(t *testing.T, input string) {
		kind, err := ParseNodeKind(input)
		if input == "folder" || input == "connection" {
			if err != nil || kind.String() != input || !kind.Valid() {
				t.Fatalf("ParseNodeKind(%q) = %q, %v", input, kind, err)
			}
			return
		}
		if !errors.Is(err, ErrInvalidNodeKind) {
			t.Fatalf("ParseNodeKind(%q) error = %v, want ErrInvalidNodeKind", input, err)
		}
	})
}

func FuzzNewRevision(f *testing.F) {
	f.Add(uint64(0))
	f.Add(uint64(1))
	f.Add(uint64(41))
	f.Add(uint64(math.MaxUint64))
	f.Fuzz(func(t *testing.T, value uint64) {
		revision, err := NewRevision(value)
		if value == 0 {
			if !errors.Is(err, ErrInvalidRevision) {
				t.Fatalf("NewRevision(0) error = %v, want ErrInvalidRevision", err)
			}
			return
		}
		if err != nil || revision.Uint64() != value || !revision.Valid() {
			t.Fatalf("NewRevision(%d) = %d, %v", value, revision.Uint64(), err)
		}

		next, err := revision.Next()
		if value == math.MaxUint64 {
			if !errors.Is(err, ErrRevisionOverflow) {
				t.Fatalf("Revision(%d).Next() error = %v, want ErrRevisionOverflow", value, err)
			}
			return
		}
		if err != nil || next.Uint64() != value+1 || revision.Uint64() != value {
			t.Fatalf("Revision(%d).Next() = %d, %v", value, next.Uint64(), err)
		}
	})
}
