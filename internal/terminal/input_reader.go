package terminal

import "io"

// CancelableInput is an input reader whose pending reads can be interrupted.
type CancelableInput interface {
	io.ReadCloser
	Cancel() bool
}
