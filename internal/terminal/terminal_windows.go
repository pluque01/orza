//go:build windows

package terminal

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

var (
	errNotTerminal  = errors.New("input and output must be terminals")
	errForeignState = errors.New("terminal state belongs to another terminal")
)

type windowsTerminalOps struct {
	isTerminal    func(int) bool
	getState      func(int) (*term.State, error)
	makeRaw       func(int) (*term.State, error)
	restore       func(int, *term.State) error
	getSize       func(int) (int, int, error)
	getInputMode  func(int) (uint32, error)
	setInputMode  func(int, uint32) error
	getOutputMode func(int) (uint32, error)
	setOutputMode func(int, uint32) error
	secretReader  secretReaderFactory
	standardInput func(*os.File) bool
}

// System is the Windows console terminal adapter.
type System struct {
	input        *os.File
	output       *os.File
	ops          windowsTerminalOps
	pollInterval time.Duration

	mu         sync.Mutex
	raw        bool
	generation uint64
}

type nativeWindowsState struct {
	owner *System
	state *term.State
}

// New returns a console adapter for input and output. Nil files select the
// process standard input and output.
func New(input, output *os.File) *System {
	if input == nil {
		input = os.Stdin
	}
	if output == nil {
		output = os.Stdout
	}
	return &System{
		input:        input,
		output:       output,
		pollInterval: 200 * time.Millisecond,
		ops: windowsTerminalOps{
			isTerminal: term.IsTerminal,
			getState:   term.GetState,
			makeRaw:    term.MakeRaw,
			restore:    term.Restore,
			getSize:    term.GetSize,
			getInputMode: func(fd int) (uint32, error) {
				var mode uint32
				err := windows.GetConsoleMode(windows.Handle(fd), &mode)
				return mode, err
			},
			setInputMode: func(fd int, mode uint32) error {
				return windows.SetConsoleMode(windows.Handle(fd), mode)
			},
			getOutputMode: func(fd int) (uint32, error) {
				var mode uint32
				err := windows.GetConsoleMode(windows.Handle(fd), &mode)
				return mode, err
			},
			setOutputMode: func(fd int, mode uint32) error {
				return windows.SetConsoleMode(windows.Handle(fd), mode)
			},
			secretReader: newUVSecretReader,
			standardInput: func(input *os.File) bool {
				return input != nil && os.Stdin != nil && input.Fd() == os.Stdin.Fd()
			},
		},
	}
}

func (t *System) Interactive() bool {
	return t != nil && t.ops.isTerminal(int(t.input.Fd())) && t.ops.isTerminal(int(t.output.Fd()))
}

// VTCapability probes the same Windows console modes required by Bubble Tea
// and restores both handles before returning.
func (t *System) VTCapability(ctx context.Context) (VTCapability, error) {
	if err := contextError(ctx); err != nil {
		return VTCapability{}, err
	}
	inputMode, err := t.ops.getInputMode(int(t.input.Fd()))
	if err != nil {
		return VTCapability{}, errors.Join(ErrRequiredVT, fmt.Errorf("inspect virtual terminal input support: %w", err))
	}
	outputMode, err := t.ops.getOutputMode(int(t.output.Fd()))
	if err != nil {
		return VTCapability{}, errors.Join(ErrRequiredVT, fmt.Errorf("inspect virtual terminal output support: %w", err))
	}

	requiredInputMode := inputMode | windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	requiredOutputMode := outputMode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING | windows.DISABLE_NEWLINE_AUTO_RETURN
	inputChanged := requiredInputMode != inputMode
	outputChanged := requiredOutputMode != outputMode
	if inputChanged {
		if err := t.ops.setInputMode(int(t.input.Fd()), requiredInputMode); err != nil {
			return VTCapability{Status: VTMissing}, nil
		}
	}
	if outputChanged {
		if err := t.ops.setOutputMode(int(t.output.Fd()), requiredOutputMode); err != nil {
			if inputChanged {
				if restoreErr := t.ops.setInputMode(int(t.input.Fd()), inputMode); restoreErr != nil {
					return VTCapability{}, fmt.Errorf("restore terminal after VT capability check: %w", restoreErr)
				}
			}
			return VTCapability{Status: VTMissing}, nil
		}
	}

	var restoreErr error
	if outputChanged {
		restoreErr = t.ops.setOutputMode(int(t.output.Fd()), outputMode)
	}
	if inputChanged {
		restoreErr = errors.Join(restoreErr, t.ops.setInputMode(int(t.input.Fd()), inputMode))
	}
	if restoreErr != nil {
		return VTCapability{}, fmt.Errorf("restore terminal after VT capability check: %w", restoreErr)
	}
	return VTCapability{Status: VTSupported}, nil
}

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
	return State{raw: t.raw, generation: t.generation, native: nativeWindowsState{owner: t, state: state}}, nil
}

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

func (t *System) Restore(ctx context.Context, state State) error {
	if err := contextError(ctx); err != nil {
		return err
	}
	native, ok := state.native.(nativeWindowsState)
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

func (t *System) ReadSecret(ctx context.Context, prompt SecretPrompt) ([]byte, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if !t.Interactive() {
		return nil, errNotTerminal
	}
	if !t.ops.standardInput(t.input) {
		return nil, errors.New("secure secret input requires the process standard console input handle")
	}
	pair, err := prepareSecretReader(t.input, t.ops.secretReader)
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}
	outputMode, err := t.ops.getOutputMode(int(t.output.Fd()))
	if err != nil {
		return nil, errors.Join(fmt.Errorf("capture terminal output mode: %w", err), closeIdleSecretReader(pair))
	}
	t.mu.Lock()
	previousRaw := t.raw
	t.mu.Unlock()
	if err := t.MakeRaw(ctx); err != nil {
		closeErr := closeIdleSecretReader(pair)
		return nil, errors.Join(err, closeErr)
	}
	vtOutputMode := outputMode | windows.ENABLE_PROCESSED_OUTPUT | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if err := t.ops.setOutputMode(int(t.output.Fd()), vtOutputMode); err != nil {
		closeErr := closeIdleSecretReader(pair)
		restoreErr := t.restoreSecretOutputMode(outputMode, previousRaw)
		return nil, errors.Join(fmt.Errorf("enable virtual terminal output: %w", err), closeErr, restoreErr)
	}
	secret, err := runSecretPrompt(ctx, prompt, t.output, "\r\n", func(context.Context) error {
		return t.restoreSecretOutputMode(outputMode, previousRaw)
	}, pair)
	if err != nil {
		return nil, fmt.Errorf("read secret: %w", err)
	}
	return secret, nil
}

func (t *System) restoreSecretOutputMode(outputMode uint32, raw bool) error {
	err := t.ops.setOutputMode(int(t.output.Fd()), outputMode)
	if err != nil {
		err = fmt.Errorf("restore terminal output mode: %w", err)
	}
	t.mu.Lock()
	t.raw = raw
	t.generation++
	t.mu.Unlock()
	return err
}

// ResizeEvents polls because Windows has no SIGWINCH equivalent for console clients.
func (t *System) ResizeEvents(ctx context.Context) (<-chan Size, error) {
	if err := contextError(ctx); err != nil {
		return nil, err
	}
	if !t.Interactive() {
		return nil, errNotTerminal
	}
	last, err := t.Size(ctx)
	if err != nil {
		return nil, err
	}
	events := make(chan Size, 1)
	go func() {
		defer close(events)
		ticker := time.NewTicker(t.pollInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				size, err := t.Size(ctx)
				if err != nil || size == last {
					continue
				}
				last = size
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
