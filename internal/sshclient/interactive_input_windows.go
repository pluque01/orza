//go:build windows

package sshclient

import (
	"fmt"
	"os"
	"time"

	"github.com/pluque01/orza/internal/terminal"
	"golang.org/x/term"
)

func newInteractiveInput(file *os.File) (*interactiveInput, error) {
	// Redirected streams do not share the interactive console's keyboard.
	if !term.IsTerminal(int(file.Fd())) {
		return &interactiveInput{reader: file}, nil
	}
	reader, err := terminal.NewCancelableInput(file)
	if err != nil {
		return nil, fmt.Errorf("prepare interactive input: %w", err)
	}

	r := &interactiveInput{reader: reader, closeReader: reader.Close}
	// CancelIoEx may run before an already admitted Read reaches ReadFile. Retry
	// until the admitted read has returned, then restore the console mode.
	r.cancel = func() {
		done := make(chan struct{})
		go func() { r.active.Wait(); close(done) }()
		for {
			reader.Cancel()
			select {
			case <-done:
				return
			case <-time.After(time.Millisecond):
			}
		}
	}
	return r, nil
}
