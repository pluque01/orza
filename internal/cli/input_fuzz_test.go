package cli

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/domain"
)

func FuzzCLISelectorInput(f *testing.F) {
	f.Add("/")
	f.Add("/clients/acme")
	f.Add("11111111111111111111111111111111")
	f.Add("-oProxyCommand=touch /tmp/pwned")
	f.Add("../relative")
	f.Fuzz(func(t *testing.T, input string) {
		selector, err := parseSelector(input)
		if err != nil {
			if strings.HasPrefix(input, "/") && !errors.Is(err, domain.ErrInvalidPath) ||
				!strings.HasPrefix(input, "/") && !errors.Is(err, domain.ErrInvalidID) {
				t.Fatalf("parseSelector(%q) error = %v", input, err)
			}
			return
		}
		if (selector.ID == "") == (selector.Path == "") {
			t.Fatalf("parseSelector(%q) = %#v", input, selector)
		}
		if selector.Path != "" && selector.Path != input || selector.ID != "" && string(selector.ID) != input {
			t.Fatalf("parseSelector(%q) changed the selector to %#v", input, selector)
		}
	})
}

func FuzzCLIHostAndCommandArguments(f *testing.F) {
	f.Add("example.test", "/prod", []byte("connection\x00show\x00/prod"))
	f.Add("-oProxyCommand=touch /tmp/pwned", "/unsafe", []byte("--json\x00connection\x00list"))
	f.Add("host\x00name", "/bad", []byte("--unknown"))
	f.Fuzz(func(t *testing.T, host, path string, encodedArgs []byte) {
		if len(host) > 2048 || len(path) > 2048 || len(encodedArgs) > 4096 {
			t.Skip()
		}
		details, err := domain.NewConnectionDetails(host, 22, "user", domain.AuthMethodAgent, "", "")
		if err == nil && details.Host() != host {
			t.Fatalf("host changed from %q to %q", host, details.Host())
		}
		if strings.HasPrefix(host, "-") && err == nil && !strings.HasPrefix(details.Host(), "-") {
			t.Fatalf("leading-dash host was interpreted as an option: %q", details.Host())
		}
		_, _, _ = splitConnectionPath(path)

		parts := strings.Split(string(encodedArgs), "\x00")
		if len(parts) > 16 {
			parts = parts[:16]
		}
		for index := range parts {
			if len(parts[index]) > 256 {
				parts[index] = parts[index][:256]
			}
		}
		var stdout, stderr bytes.Buffer
		root := NewRoot(RootConfig{Version: "fuzz", Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
		root.SetArgs(parts)
		commandErr := root.ExecuteContext(context.Background())
		if commandErr != nil {
			if writeErr := WriteError(io.Discard, true, commandErr); writeErr != nil {
				t.Fatalf("WriteError() error = %v", writeErr)
			}
		}
		if stdout.Len()+stderr.Len() > 1<<20 {
			t.Fatalf("bounded command input produced %d output bytes", stdout.Len()+stderr.Len())
		}
	})
}
