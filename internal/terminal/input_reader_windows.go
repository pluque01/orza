//go:build windows

package terminal

import (
	"errors"
	"io"
	"os"

	uv "github.com/charmbracelet/ultraviolet"
)

// NewCancelableInput prepares a console input reader that can be interrupted.
func NewCancelableInput(input io.Reader) (CancelableInput, error) {
	file, ok := input.(*os.File)
	if !ok || os.Stdin == nil || file.Fd() != os.Stdin.Fd() {
		return nil, errors.New("input is not the standard console")
	}
	reader, err := uv.NewCancelReader(input)
	if err != nil {
		return nil, err
	}
	if reader == nil {
		return nil, errors.New("no input reader")
	}
	if !isUltravioletNativeWindowsReader(reader) {
		return nil, errors.Join(errors.New("input cannot be canceled"), reader.Close())
	}
	return reader, nil
}
