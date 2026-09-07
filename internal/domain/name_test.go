package domain

import (
	"errors"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestNewNameValidatesTrimmedValueButPreservesExactInput(t *testing.T) {
	t.Parallel()

	for _, input := range []string{"prod", " Prod ", "caf\u00e9", "\u65e5\u672c"} {
		name, err := NewName(input)
		if err != nil {
			t.Fatalf("NewName(%q) error = %v", input, err)
		}
		if got := name.String(); got != input {
			t.Fatalf("NewName(%q).String() = %q", input, got)
		}
	}
}

func TestNewNameRejectsInvalidValuesWithoutEchoingThem(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input string
	}{
		{name: "empty", input: ""},
		{name: "ASCII whitespace", input: " \t "},
		{name: "Unicode whitespace", input: "\u2003\u00a0"},
		{name: "slash", input: "prod/backup"},
		{name: "NUL", input: "prod\x00backup"},
		{name: "newline", input: "prod\nbackup"},
		{name: "Unicode control", input: "prod\u0085backup"},
		{name: "invalid UTF-8", input: string([]byte{'p', 0xff})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := NewName(tt.input)
			if !errors.Is(err, ErrInvalidName) {
				t.Fatalf("NewName(%q) error = %v, want ErrInvalidName", tt.input, err)
			}
			var validationError *ValidationError
			if !errors.As(err, &validationError) || validationError.Field != "name" {
				t.Fatalf("NewName(%q) error = %#v, want name ValidationError", tt.input, err)
			}
			if tt.input != "" && strings.Contains(err.Error(), tt.input) {
				t.Fatalf("error %q exposes rejected input", err)
			}
		})
	}
}

func FuzzNewName(f *testing.F) {
	f.Add("prod")
	f.Add(" Prod ")
	f.Add("bad/name")
	f.Add("bad\x00name")
	f.Add("")
	f.Fuzz(func(t *testing.T, input string) {
		name, err := NewName(input)
		valid := utf8.ValidString(input) && strings.TrimSpace(input) != "" && !strings.ContainsRune(input, '/')
		for _, character := range input {
			valid = valid && !unicode.IsControl(character)
		}

		if !valid {
			if !errors.Is(err, ErrInvalidName) {
				t.Fatalf("NewName(%q) error = %v, want ErrInvalidName", input, err)
			}
			return
		}
		if err != nil {
			t.Fatalf("NewName(%q) error = %v", input, err)
		}
		if name.String() != input {
			t.Fatalf("NewName(%q).String() = %q", input, name)
		}
	})
}
