package terminal

import (
	"context"
	"fmt"
	"io"
	"sync"
)

// Operation identifies a terminal operation for calls and faults.
type Operation string

const (
	OperationInteractive  Operation = "interactive"
	OperationCapture      Operation = "capture"
	OperationVTCapability Operation = "vt_capability"
	OperationSize         Operation = "size"
	OperationMakeRaw      Operation = "make_raw"
	OperationRestore      Operation = "restore"
	OperationReadSecret   Operation = "read_secret"
	OperationResizeEvents Operation = "resize_events"
)

// Call records one terminal operation without recording secret input.
type Call struct {
	Operation Operation
	Prompt    SecretPrompt
	State     State
}

type secretResponse struct {
	secret []byte
	err    error
}

// Fake is a deterministic, concurrency-safe Terminal. Its zero value is
// usable; configure size, prompts, and resize events before exercising it.
type Fake struct {
	mu          sync.Mutex
	interactive bool
	capability  VTCapability
	state       State
	size        Size
	responses   []secretResponse
	resizes     []Size
	faults      map[Operation]error
	calls       []Call
}

// NewFake returns an interactive fake with the provided initial size.
func NewFake(size Size) *Fake {
	return &Fake{interactive: true, capability: VTCapability{Status: VTSupported}, size: size}
}

// SetInteractive changes whether the fake represents an interactive terminal.
func (f *Fake) SetInteractive(interactive bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.interactive = interactive
}

// Interactive reports the configured interactivity.
func (f *Fake) Interactive() bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, Call{Operation: OperationInteractive})
	return f.interactive
}

// SetVTCapability changes the result returned by VTCapability.
func (f *Fake) SetVTCapability(capability VTCapability) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.capability = capability
}

// VTCapability returns the configured required-VT capability report.
func (f *Fake) VTCapability(ctx context.Context) (VTCapability, error) {
	if err := ctx.Err(); err != nil {
		return VTCapability{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, Call{Operation: OperationVTCapability})
	if err := f.faults[OperationVTCapability]; err != nil {
		return VTCapability{}, err
	}
	return f.capability, nil
}

// SetSize changes the size returned by Size.
func (f *Fake) SetSize(size Size) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.size = size
}

// SetRaw changes the fake's current raw-mode state.
func (f *Fake) SetRaw(raw bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.state.raw = raw
	f.state.generation++
}

// CurrentState returns the current state for test assertions.
func (f *Fake) CurrentState() State {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.state
}

// QueueSecret adds one response to the no-echo prompt queue. The secret is copied.
func (f *Fake) QueueSecret(secret []byte, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.responses = append(f.responses, secretResponse{secret: clone(secret), err: err})
}

// QueueResize adds sizes to the next finite ResizeEvents stream.
func (f *Fake) QueueResize(sizes ...Size) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resizes = append(f.resizes, sizes...)
}

// SetFault makes every subsequent operation of the given kind return err.
// Passing nil clears the fault.
func (f *Fake) SetFault(operation Operation, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err == nil {
		delete(f.faults, operation)
		return
	}
	if f.faults == nil {
		f.faults = make(map[Operation]error)
	}
	f.faults[operation] = err
}

// Calls returns a snapshot of calls in invocation order.
func (f *Fake) Calls() []Call {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]Call(nil), f.calls...)
}

// Capture returns the current terminal state.
func (f *Fake) Capture(ctx context.Context) (State, error) {
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return State{}, err
	}
	f.calls = append(f.calls, Call{Operation: OperationCapture})
	if err := f.faults[OperationCapture]; err != nil {
		return State{}, err
	}
	return f.state, nil
}

// Size returns the configured terminal size.
func (f *Fake) Size(ctx context.Context) (Size, error) {
	if err := ctx.Err(); err != nil {
		return Size{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Size{}, err
	}
	f.calls = append(f.calls, Call{Operation: OperationSize})
	if err := f.faults[OperationSize]; err != nil {
		return Size{}, err
	}
	return f.size, nil
}

// MakeRaw changes the current state to raw mode.
func (f *Fake) MakeRaw(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	f.calls = append(f.calls, Call{Operation: OperationMakeRaw})
	if err := f.faults[OperationMakeRaw]; err != nil {
		return err
	}
	f.state.raw = true
	f.state.generation++
	return nil
}

// Restore returns the fake to a previously captured state.
func (f *Fake) Restore(ctx context.Context, state State) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return err
	}
	f.calls = append(f.calls, Call{Operation: OperationRestore, State: state})
	if err := f.faults[OperationRestore]; err != nil {
		return err
	}
	f.state = state
	return nil
}

// ReadSecret consumes and copies the next configured response.
func (f *Fake) ReadSecret(ctx context.Context, prompt SecretPrompt) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.calls = append(f.calls, Call{Operation: OperationReadSecret, Prompt: prompt})
	if err := f.faults[OperationReadSecret]; err != nil {
		return nil, err
	}
	if len(f.responses) == 0 {
		return nil, ErrNoSecretResponse
	}
	response := f.responses[0]
	f.responses[0] = secretResponse{}
	f.responses = f.responses[1:]
	secret := clone(response.secret)
	wipe(response.secret)
	return secret, response.err
}

// ResizeEvents returns the queued resize sequence as a finite stream.
func (f *Fake) ResizeEvents(ctx context.Context) (<-chan Size, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.calls = append(f.calls, Call{Operation: OperationResizeEvents})
	if err := f.faults[OperationResizeEvents]; err != nil {
		return nil, err
	}

	events := make(chan Size, len(f.resizes))
	for _, size := range f.resizes {
		events <- size
		f.size = size
	}
	close(events)
	f.resizes = nil
	return events, nil
}

// Format prevents fmt from traversing queued secret responses.
func (f *Fake) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, "terminal.Fake{redacted}")
}

func clone(value []byte) []byte {
	return append([]byte(nil), value...)
}

func wipe(value []byte) {
	for i := range value {
		value[i] = 0
	}
}
