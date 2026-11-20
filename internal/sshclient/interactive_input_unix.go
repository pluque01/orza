//go:build !windows

package sshclient

import (
	"fmt"
	"os"

	"github.com/pluque01/orza/internal/terminal"
)

func newInteractiveInput(file *os.File) (*interactiveInput, error) {
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("inspect interactive input: %w", err)
	}
	// Regular files cannot block waiting for terminal input and epoll rejects them.
	if info.Mode().IsRegular() {
		return &interactiveInput{reader: file}, nil
	}
	// The null device returns EOF without waiting for input.
	if nullInfo, statErr := os.Stat(os.DevNull); statErr == nil && os.SameFile(info, nullInfo) {
		return &interactiveInput{reader: file}, nil
	}
	reader, err := terminal.NewCancelableInput(file)
	if err != nil {
		return nil, fmt.Errorf("prepare interactive input: %w", err)
	}
	return &interactiveInput{reader: reader, cancel: func() { reader.Cancel() }, closeReader: reader.Close}, nil
}
