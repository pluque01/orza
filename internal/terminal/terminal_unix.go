//go:build !windows

package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/term"
)

var (
	errNotTerminal  = errors.New("input and output must be terminals")
	errForeignState = errors.New("terminal state belongs to another terminal")
)

type unixTerminalOps struct {
	isTerminal   func(int) bool
	getState     func(int) (*term.State, error)
	makeRaw      func(int) (*term.State, error)
	restore      func(int, *term.State) error
	getSize      func(int) (int, int, error)
	secretReader secretReaderFactory
	notify       func(chan<- os.Signal, ...os.Signal)
	stop         func(chan<- os.Signal)
}

// System is the operating system terminal adapter.
type System struct {
	input  *os.File
	output *os.File
	ops    unixTerminalOps

	mu         sync.Mutex
	raw        bool
	generation uint64
}

type nativeUnixState struct {
	owner *System
	state *term.State
}

// New returns a terminal adapter for input and output. Nil files select the
// process standard input and output.
func New(input, output *os.File) *System {
	if input == nil {
		input = os.Stdin
	}
	if output == nil {
		output = os.Stdout
	}
	return &System{
		input:  input,
		output: output,
		ops: unixTerminalOps{
			isTerminal:   term.IsTerminal,
			getState:     term.GetState,
			makeRaw:      term.MakeRaw,
			restore:      term.Restore,
			getSize:      term.GetSize,
			secretReader: newUVSecretReader,
			notify:       signal.Notify,
			stop:         signal.Stop,
		},
	}
}

// Interactive reports whether both ends needed by an interactive session are terminals.
func (t *System) Interactive() bool {
	return t != nil && t.ops.isTerminal(int(t.input.Fd())) && t.ops.isTerminal(int(t.output.Fd()))
}

// VTCapability is unverified because Unix terminal devices expose no reliable
// query for the emulator's VT input and output protocol support.
func (t *System) VTCapability(ctx context.Context) (VTCapability, error) {
	if err := contextError(ctx); err != nil {
		return VTCapability{}, err
	}
	return VTCapability{Status: VTUnverified}, nil
}

// Capture snapshots the input terminal state for a later Restore.
func (t *System) Capture(ctx context.Context) (State, error) {
	if err := contextError(ctx); err != nil {
		return State{}, err
	}
	state, err := t.ops.getState(int(t.input.Fd()))
	if err != nil {
		return State{}, fmt.Errorf("capture terminal: %w", err)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return State{
		raw:        t.raw,
		generation: t.generation,
		native:     nativeUnixState{owner: t, state: state},
	}, nil
}

// Size returns the output terminal dimensions in character cells.
func (t *System) Size(ctx context.Context) (Size, error) {
	if err := contextError(ctx); err != nil {
		return Size{}, err
	}
	columns, rows, err := t.ops.getSize(int(t.output.Fd()))
	if err != nil {
		return Size{}, fmt.Errorf("read terminal size: %w", err)
	}
	size := Size{Columns: columns, Rows: rows}
	if !size.Valid() {
		return Size{}, fmt.Errorf("read terminal size: invalid dimensions %dx%d", columns, rows)
	}
	return size, nil
}

// MakeRaw switches input to raw mode.
func (t *System) MakeRaw(ctx context.Context) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	if _, err := t.ops.makeRaw(int(t.input.Fd())); err != nil {
		return fmt.Errorf("make terminal raw: %w", err)
	}
	t.mu.Lock()
	t.raw = true
	t.generation++
	t.mu.Unlock()
	return nil
}

// Restore applies a state previously captured from this adapter.
func (t *System) Restore(ctx context.Context, state State) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	native, ok := state.native.(nativeUnixState)
	if !ok || native.owner != t || native.state == nil {
		return errForeignState
	}
	if err := t.ops.restore(int(t.input.Fd()), native.state); err != nil {
		return fmt.Errorf("restore terminal: %w", err)
	}
	t.mu.Lock()
	t.raw = state.raw
	t.generation++
	t.mu.Unlock()
	return nil
}

// ReadSecret runs a no-echo event editor and restores the captured terminal state before returning.
func (t *System) ReadSecret(ctx context.Context, prompt SecretPrompt) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if !t.Interactive() {
		return nil, errNotTerminal
	}
	pair, err := prepareSecretReader(t.input, t.ops.secretReader)
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}
	state, err := t.Capture(ctx)
	if err != nil {
		return nil, errors.Join(err, closeIdleSecretReader(pair))
	}
	if err := t.MakeRaw(ctx); err != nil {
		closeErr := closeIdleSecretReader(pair)
		restoreErr := t.Restore(context.WithoutCancel(ctx), state)
		return nil, errors.Join(err, closeErr, restoreErr)
	}
	secret, err := runSecretPrompt(ctx, prompt, t.output, "\n", func(cleanupCtx context.Context) error {
		return t.Restore(cleanupCtx, state)
	}, pair)
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}
	return secret, nil
}

// ResizeEvents converts SIGWINCH notifications into current terminal sizes.
func (t *System) ResizeEvents(ctx context.Context) (<-chan Size, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if !t.Interactive() {
		return nil, errNotTerminal
	}
	signals := make(chan os.Signal, 1)
	t.ops.notify(signals, syscall.SIGWINCH)
	events := make(chan Size, 1)
	go func() {
		defer close(events)
		defer t.ops.stop(signals)
		for {
			select {
			case <-ctx.Done():
				return
			case _, ok := <-signals:
				if !ok {
					return
				}
				size, err := t.Size(ctx)
				if err != nil {
					continue
				}
				select {
				case events <- size:
				default:
					select {
					case <-events:
					default:
					}
					select {
					case events <- size:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()
	return events, nil
}

func contextError(ctx context.Context) error {
	if ctx == nil {
		return errors.New("nil context")
	}
	return ctx.Err()
}
