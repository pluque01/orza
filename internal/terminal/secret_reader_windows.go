//go:build windows

package terminal

import (
	"errors"
	"io"
	"os"
	"reflect"

	uv "github.com/charmbracelet/ultraviolet"
)

const ultravioletPackagePath = "github.com/charmbracelet/ultraviolet"

type windowsCancelReaderFactory func(io.Reader) (secretCancelReader, bool, error)

func newUVSecretReader(input io.Reader) (secretReaderPair, error) {
	return newWindowsUVSecretReader(input, func(input io.Reader) (secretCancelReader, bool, error) {
		reader, err := uv.NewCancelReader(input)
		return reader, isUltravioletNativeWindowsReader(reader), err
	})
}

func newWindowsUVSecretReader(input io.Reader, newCancelReader windowsCancelReaderFactory) (secretReaderPair, error) {
	file, ok := input.(*os.File)
	if !ok || os.Stdin == nil || file.Fd() != os.Stdin.Fd() {
		return secretReaderPair{}, errSecretReaderUnsafe
	}
	reader, native, err := newCancelReader(input)
	if err != nil {
		if reader != nil {
			if closeErr := reader.Close(); closeErr != nil {
				return secretReaderPair{}, errors.Join(errSecretInput, errSecretReaderClose)
			}
		}
		return secretReaderPair{}, errSecretInput
	}
	if reader == nil {
		return secretReaderPair{}, errSecretInput
	}
	if !native {
		if err := reader.Close(); err != nil {
			return secretReaderPair{}, errors.Join(errSecretReaderUnsafe, errSecretReaderClose)
		}
		return secretReaderPair{}, errSecretReaderUnsafe
	}
	return newTerminalSecretReader(reader), nil
}

func isUltravioletNativeWindowsReader(reader secretCancelReader) bool {
	readerType := reflect.TypeOf(reader)
	return readerType != nil && readerType.Kind() == reflect.Pointer &&
		readerType.Elem().PkgPath() == ultravioletPackagePath && readerType.Elem().Name() == "conInputReader"
}
