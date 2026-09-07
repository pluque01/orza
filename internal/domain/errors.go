package domain

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidID        = errors.New("invalid ID")
	ErrInvalidName      = errors.New("invalid name")
	ErrInvalidNodeKind  = errors.New("invalid node kind")
	ErrInvalidRevision  = errors.New("invalid revision")
	ErrRevisionOverflow = errors.New("revision overflow")
	ErrInvalidPath      = errors.New("invalid logical path")
)

// ValidationError identifies a field without retaining or displaying its rejected value.
type ValidationError struct {
	Field string
	Err   error
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %v", e.Field, e.Err)
}

func (e *ValidationError) Unwrap() error {
	return e.Err
}

func invalid(field string, err error) error {
	return &ValidationError{Field: field, Err: err}
}
