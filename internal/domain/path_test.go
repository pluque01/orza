package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestParseLogicalPath(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		segments []string
	}{
		{input: "/"},
		{input: "/clientes", segments: []string{"clientes"}},
		{input: "/clientes/Acme/ prod ", segments: []string{"clientes", "Acme", " prod "}},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			t.Parallel()
			path, err := ParseLogicalPath(tt.input)
			if err != nil {
				t.Fatalf("ParseLogicalPath(%q) error = %v", tt.input, err)
			}
			if path.String() != tt.input {
				t.Fatalf("ParseLogicalPath(%q).String() = %q", tt.input, path)
			}
			if path.IsRoot() != (tt.input == "/") {
				t.Fatalf("ParseLogicalPath(%q).IsRoot() = %t", tt.input, path.IsRoot())
			}
			segments := path.Segments()
			if len(segments) != len(tt.segments) {
				t.Fatalf("Segments() length = %d, want %d", len(segments), len(tt.segments))
			}
			for index, want := range tt.segments {
				if segments[index].String() != want {
					t.Errorf("Segments()[%d] = %q, want %q", index, segments[index], want)
				}
			}
		})
	}
}

func TestParseLogicalPathRejectsNonCanonicalPaths(t *testing.T) {
	t.Parallel()

	for _, input := range []string{
		"", "relative", "clientes/acme", "//", "/clientes/", "/clientes//acme",
		"/.", "/..", "/clientes/./acme", "/clientes/../acme", "/bad\x00name", "/bad\nname",
	} {
		_, err := ParseLogicalPath(input)
		if !errors.Is(err, ErrInvalidPath) {
			t.Errorf("ParseLogicalPath(%q) error = %v, want ErrInvalidPath", input, err)
		}
		if input != "" && strings.Contains(err.Error(), input) {
			t.Errorf("error %q exposes rejected path", err)
		}
	}
}

func TestLogicalPathConstructionAndCopies(t *testing.T) {
	t.Parallel()

	clients, err := NewName("clientes")
	if err != nil {
		t.Fatal(err)
	}
	acme, err := NewName("Acme")
	if err != nil {
		t.Fatal(err)
	}
	path, err := NewLogicalPath(clients, acme)
	if err != nil {
		t.Fatalf("NewLogicalPath() error = %v", err)
	}
	if path.String() != "/clientes/Acme" {
		t.Fatalf("NewLogicalPath().String() = %q", path)
	}

	segments := path.Segments()
	segments[0] = acme
	if path.String() != "/clientes/Acme" {
		t.Fatalf("mutating Segments() result changed path to %q", path)
	}

	root, err := NewLogicalPath()
	if err != nil || root.String() != "/" {
		t.Fatalf("NewLogicalPath() = %q, %v", root, err)
	}
	dot, err := NewName(".")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewLogicalPath(dot); !errors.Is(err, ErrInvalidPath) {
		t.Fatalf("NewLogicalPath(dot) error = %v, want ErrInvalidPath", err)
	}
}

func FuzzParseLogicalPath(f *testing.F) {
	f.Add("/")
	f.Add("/clientes/Acme")
	f.Add("relative")
	f.Add("/clientes//acme")
	f.Add("/clientes/../acme")
	f.Fuzz(func(t *testing.T, input string) {
		path, err := ParseLogicalPath(input)
		if err != nil {
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("ParseLogicalPath(%q) error = %v, want ErrInvalidPath", input, err)
			}
			return
		}

		if path.String() != input || !strings.HasPrefix(input, "/") {
			t.Fatalf("successful ParseLogicalPath(%q) produced %q", input, path)
		}
		if input == "/" {
			return
		}
		for _, segment := range path.Segments() {
			if segment.String() == "." || segment.String() == ".." || segment.String() == "" {
				t.Fatalf("ParseLogicalPath(%q) accepted segment %q", input, segment)
			}
		}
	})
}
