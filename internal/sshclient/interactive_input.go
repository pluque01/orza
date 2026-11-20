package sshclient

import (
	"io"
	"sync"
)

// interactiveInput serializes teardown with SSH's input-copy goroutine. The
// underlying stdin remains owned by the application, not the SSH session.
type interactiveInput struct {
	reader      io.Reader
	cancel      func()
	closeReader func() error
	mu          sync.Mutex
	closed      bool
	active      sync.WaitGroup
}

func (r *interactiveInput) Read(p []byte) (int, error) {
	r.mu.Lock()
	if r.closed {
		r.mu.Unlock()
		return 0, io.EOF
	}
	r.active.Add(1)
	r.mu.Unlock()
	defer r.active.Done()
	return r.reader.Read(p)
}

func (r *interactiveInput) Close() error {
	r.mu.Lock()
	r.closed = true
	r.mu.Unlock()
	if r.cancel != nil {
		r.cancel()
	}
	r.active.Wait()
	if r.closeReader != nil {
		return r.closeReader()
	}
	return nil
}
