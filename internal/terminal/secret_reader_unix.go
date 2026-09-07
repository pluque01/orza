//go:build !windows

package terminal

import (
	"io"
	"runtime"

	uv "github.com/charmbracelet/ultraviolet"
)

type secretInputFile interface {
	io.ReadWriteCloser
	Fd() uintptr
	Name() string
}

func newUVSecretReader(input io.Reader) (secretReaderPair, error) {
	if !unixSecretCancellationGuaranteed(input) {
		return secretReaderPair{}, errSecretReaderUnsafe
	}
	reader, err := uv.NewCancelReader(input)
	if err != nil {
		return secretReaderPair{}, errSecretInput
	}
	return newTerminalSecretReader(reader), nil
}

func unixSecretCancellationGuaranteed(input io.Reader) bool {
	file, ok := input.(secretInputFile)
	if !ok {
		return false
	}
	switch runtime.GOOS {
	case "linux":
		return true
	case "darwin", "freebsd", "netbsd", "openbsd", "dragonfly":
		return file.Name() != "/dev/tty" || file.Fd() < 1024
	case "solaris":
		return file.Fd() < 1024
	default:
		return false
	}
}
