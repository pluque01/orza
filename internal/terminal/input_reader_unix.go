//go:build !windows

package terminal

import (
	"errors"
	"io"

	uv "github.com/charmbracelet/ultraviolet"
)

// NewCancelableInput prepares a file-backed input reader that can be interrupted.
func NewCancelableInput(input io.Reader) (CancelableInput, error) {
	if !unixSecretCancellationGuaranteed(input) {
		return nil, errors.New("input cannot be canceled on this platform")
	}
	reader, err := uv.NewCancelReader(input)
	if err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, errors.New("no input reader")
	}
	return reader, nil
}
