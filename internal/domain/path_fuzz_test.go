package domain

import (
	"errors"
	"strings"
	"testing"
)

func FuzzLogicalPathRoundTrip(f *testing.F) {
	f.Add("/")
	f.Add("/clients/acme/prod")
	f.Add("/caf\u00e9/\u65e5\u672c")
	f.Add("/../server")
	f.Add("//server")
	f.Fuzz(func(t *testing.T, input string) {
		path, err := ParseLogicalPath(input)
		if err != nil {
			if !errors.Is(err, ErrInvalidPath) {
				t.Fatalf("ParseLogicalPath(%q) error = %v", input, err)
			}
			return
		}

		encoded, err := path.MarshalText()
		if err != nil {
			t.Fatalf("MarshalText() error = %v", err)
		}
		if string(encoded) != input {
			t.Fatalf("path round trip = %q, want %q", encoded, input)
		}
		rebuilt, err := NewLogicalPath(path.Segments()...)
		if err != nil || rebuilt.String() != input {
			t.Fatalf("NewLogicalPath(Segments()) = %q, %v", rebuilt, err)
		}
	})
}

func FuzzNameAsLogicalPathSegment(f *testing.F) {
	f.Add("production")
	f.Add(" name with spaces ")
	f.Add(".")
	f.Add("..")
	f.Add("bad/name")
	f.Add("bad\x00name")
	f.Fuzz(func(t *testing.T, input string) {
		name, nameErr := NewName(input)
		if nameErr != nil {
			if !errors.Is(nameErr, ErrInvalidName) {
				t.Fatalf("NewName(%q) error = %v", input, nameErr)
			}
			return
		}
		if name.String() != input {
			t.Fatalf("NewName(%q).String() = %q", input, name)
		}

		path, pathErr := NewLogicalPath(name)
		if input == "." || input == ".." {
			if !errors.Is(pathErr, ErrInvalidPath) {
				t.Fatalf("NewLogicalPath(%q) error = %v", input, pathErr)
			}
			return
		}
		if pathErr != nil {
			t.Fatalf("NewLogicalPath(%q) error = %v", input, pathErr)
		}
		if path.String() != "/"+input || len(path.Segments()) != 1 || path.Segments()[0].String() != input {
			t.Fatalf("segment round trip for %q produced %q", input, path)
		}
		if strings.Contains(path.String()[1:], "/") {
			t.Fatalf("accepted name %q introduced another path segment", input)
		}
	})
}
