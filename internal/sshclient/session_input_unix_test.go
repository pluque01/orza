//go:build linux || darwin

package sshclient

import (
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/terminal"
)

func TestInteractiveSessionStopsReadingInputOnReturn(t *testing.T) {
	input, keyboard, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close() })
	t.Cleanup(func() { _ = keyboard.Close() })

	firstRead := make(chan error, 1)
	pendingRead := make(chan error, 1)
	remote := &fakeSession{}
	remote.shell = func() error {
		go func() {
			var key [1]byte
			_, err := io.ReadFull(remote.stdin, key[:])
			if err == nil && key[0] != 's' {
				err = fmt.Errorf("shell input = %q, want s", key)
			}
			firstRead <- err
			_, err = remote.stdin.Read(key[:])
			pendingRead <- err
		}()
		return nil
	}
	remote.wait = func() error { return <-firstRead }
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	client := successfulClient(local, &fakeTransport{session: remote}, &fakeConn{})
	client.options.Stdin = input
	if _, err := io.WriteString(keyboard, "s"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.Run(t.Context(), testRequest(local)); err != nil {
		t.Fatal(err)
	}

	select {
	case err := <-pendingRead:
		if err == nil {
			t.Fatal("the completed session could still read from terminal input")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the completed session still has a blocked terminal read")
	}
	if _, err := io.WriteString(keyboard, "q"); err != nil {
		t.Fatal(err)
	}
	var key [1]byte
	if _, err := io.ReadFull(input, key[:]); err != nil || key[0] != 'q' {
		t.Fatalf("first input after the session = %q, %v, want q", key, err)
	}
}
