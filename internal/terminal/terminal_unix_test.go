//go:build !windows

package terminal

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

func TestSystemCaptureRawRestore(t *testing.T) {
	input, output := testFiles(t)
	system := New(input, output)
	native := new(term.State)
	var restored *term.State
	system.ops.isTerminal = func(int) bool { return true }
	system.ops.getState = func(int) (*term.State, error) { return native, nil }
	system.ops.makeRaw = func(int) (*term.State, error) { return native, nil }
	system.ops.restore = func(_ int, state *term.State) error {
		restored = state
		return nil
	}

	state, err := system.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := system.MakeRaw(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := system.Restore(context.Background(), state); err != nil {
		t.Fatal(err)
	}
	if restored != native {
		t.Fatal("Restore did not receive captured native state")
	}
	if system.raw {
		t.Fatal("Restore did not reset tracked raw mode")
	}
}

func TestSystemVTCapabilityIsUnverifiedWithoutTERMGuess(t *testing.T) {
	input, output := testFiles(t)
	system := New(input, output)
	t.Setenv("TERM", "definitely-not-a-terminal-database-entry")

	capability, err := system.VTCapability(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capability != (VTCapability{Status: VTUnverified}) {
		t.Fatalf("VTCapability() = %+v, want unverified", capability)
	}
}

func TestSystemRejectsForeignState(t *testing.T) {
	input, output := testFiles(t)
	first := New(input, output)
	second := New(input, output)
	first.ops.getState = func(int) (*term.State, error) { return new(term.State), nil }
	state, err := first.Capture(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := second.Restore(context.Background(), state); !errors.Is(err, errForeignState) {
		t.Fatalf("Restore error = %v, want %v", err, errForeignState)
	}
}

func TestSystemReadSecretUsesEventEditorAndRestores(t *testing.T) {
	input, output := testFiles(t)
	system := New(input, output)
	native := new(term.State)
	restored := false
	system.ops.isTerminal = func(int) bool { return true }
	system.ops.getState = func(int) (*term.State, error) { return native, nil }
	system.ops.makeRaw = func(int) (*term.State, error) { return native, nil }
	system.ops.restore = func(_ int, got *term.State) error {
		restored = got == native
		return nil
	}
	order := new(callOrder)
	system.ops.secretReader = func(io.Reader) (secretReaderPair, error) {
		return testSecretReaderPair(
			&fakeSecretCancelReader{order: order},
			&fakeSecretStreamer{order: order, events: []uv.Event{
				uv.PasteEvent{Content: "canary-secret"},
				uv.KeyPressEvent(uv.Key{Code: uv.KeyEnter}),
			}},
		), nil
	}

	secret, err := system.ReadSecret(context.Background(), SecretPrompt{Message: "Password: "})
	if err != nil {
		t.Fatal(err)
	}
	if string(secret) != "canary-secret" {
		t.Fatal("ReadSecret did not return the submitted value")
	}
	wipe(secret)
	contents, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	if got := string(contents); strings.Contains(got, "canary-secret") || !strings.Contains(got, ansi.SetModeBracketedPaste) ||
		!strings.Contains(got, ansi.ResetModeBracketedPaste) || !strings.HasSuffix(got, "\n") {
		t.Fatal("prompt output leaked input or omitted terminal cleanup")
	}
	if !restored || system.raw {
		t.Fatal("ReadSecret did not restore captured raw state")
	}
}

func TestSystemReadSecretFailsClosedWithoutCancellationContract(t *testing.T) {
	input, output := testFiles(t)
	system := New(input, output)
	system.ops.isTerminal = func(int) bool { return true }
	order := new(callOrder)
	system.ops.secretReader = func(io.Reader) (secretReaderPair, error) {
		return secretReaderPair{
			cancelReader: &fakeSecretCancelReader{order: order},
			streamer:     &fakeSecretStreamer{order: order},
		}, nil
	}
	rawCalled := false
	system.ops.makeRaw = func(int) (*term.State, error) {
		rawCalled = true
		return nil, nil
	}

	secret, err := system.ReadSecret(context.Background(), SecretPrompt{Message: "Password: "})
	if err == nil || secret != nil || rawCalled {
		t.Fatal("uncancelable reader did not fail closed before raw mode")
	}
	if !errors.Is(err, errSecretReaderUnsafe) {
		t.Fatalf("ReadSecret error = %v, want cancellation safety error", err)
	}
	if calls := order.snapshot(); len(calls) != 1 || calls[0] != "close" {
		t.Fatalf("preflight cleanup calls = %v, want [close]", calls)
	}
	contents, readErr := os.ReadFile(output.Name())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(contents) != 0 {
		t.Fatal("uncancelable reader wrote a prompt or terminal sequence")
	}
}

func TestSystemReadSecretSetupFailureOccursBeforeRawOrOutput(t *testing.T) {
	input, output := testFiles(t)
	system := New(input, output)
	system.ops.isTerminal = func(int) bool { return true }
	rawCalled := false
	system.ops.makeRaw = func(int) (*term.State, error) { rawCalled = true; return nil, nil }
	system.ops.secretReader = func(io.Reader) (secretReaderPair, error) {
		return secretReaderPair{}, errors.New("injected setup failure")
	}

	secret, err := system.ReadSecret(context.Background(), SecretPrompt{Message: "Password: "})
	if err == nil || secret != nil || rawCalled || system.raw {
		t.Fatal("reader setup failure did not fail before raw mode")
	}
	contents, readErr := os.ReadFile(output.Name())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(contents) != 0 || strings.Contains(err.Error(), "injected setup failure") {
		t.Fatal("reader setup failure wrote output or exposed internal error")
	}
}

func TestSystemReadSecretMakeRawFailureAttemptsRestore(t *testing.T) {
	input, output := testFiles(t)
	system := New(input, output)
	native := new(term.State)
	restored := false
	system.ops.isTerminal = func(int) bool { return true }
	system.ops.getState = func(int) (*term.State, error) { return native, nil }
	system.ops.makeRaw = func(int) (*term.State, error) { return nil, errors.New("raw failure") }
	system.ops.restore = func(int, *term.State) error { restored = true; return nil }
	system.ops.secretReader = func(io.Reader) (secretReaderPair, error) {
		order := new(callOrder)
		return testSecretReaderPair(&fakeSecretCancelReader{order: order}, &fakeSecretStreamer{order: order}), nil
	}

	secret, err := system.ReadSecret(context.Background(), SecretPrompt{Message: "Password: "})
	if err == nil || secret != nil || !restored {
		t.Fatal("raw-mode failure did not attempt restoration")
	}
}

func TestSystemResizeEventsCoalesceAndStop(t *testing.T) {
	input, output := testFiles(t)
	system := New(input, output)
	system.ops.isTerminal = func(int) bool { return true }
	sizes := []Size{{Columns: 100, Rows: 30}, {Columns: 120, Rows: 40}}
	system.ops.getSize = func(int) (int, int, error) {
		size := sizes[0]
		sizes = sizes[1:]
		return size.Columns, size.Rows, nil
	}
	notified := make(chan chan<- os.Signal, 1)
	stopped := make(chan struct{}, 1)
	system.ops.notify = func(channel chan<- os.Signal, signals ...os.Signal) {
		if len(signals) != 1 || signals[0] != syscall.SIGWINCH {
			t.Fatalf("signals = %v", signals)
		}
		notified <- channel
	}
	system.ops.stop = func(chan<- os.Signal) { stopped <- struct{}{} }
	ctx, cancel := context.WithCancel(context.Background())
	events, err := system.ResizeEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	signalChannel := <-notified
	signalChannel <- syscall.SIGWINCH
	if got := receiveSize(t, events); got != (Size{Columns: 100, Rows: 30}) {
		t.Fatalf("first size = %v", got)
	}
	signalChannel <- syscall.SIGWINCH
	if got := receiveSize(t, events); got != (Size{Columns: 120, Rows: 40}) {
		t.Fatalf("second size = %v", got)
	}
	cancel()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("signal notifications were not stopped")
	}
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("resize stream remains open")
		}
	case <-time.After(time.Second):
		t.Fatal("resize stream was not closed")
	}
}

func TestSystemHonorsCanceledContextBeforeNativeCall(t *testing.T) {
	input, output := testFiles(t)
	system := New(input, output)
	called := false
	system.ops.makeRaw = func(int) (*term.State, error) {
		called = true
		return nil, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := system.MakeRaw(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("MakeRaw error = %v", err)
	}
	if called {
		t.Fatal("native operation called after cancellation")
	}
}

func testFiles(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	input, err := os.CreateTemp(t.TempDir(), "input")
	if err != nil {
		t.Fatal(err)
	}
	output, err := os.CreateTemp(t.TempDir(), "output")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = input.Close()
		_ = output.Close()
	})
	return input, output
}

func receiveSize(t *testing.T, events <-chan Size) Size {
	t.Helper()
	select {
	case size := <-events:
		return size
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for terminal size")
		return Size{}
	}
}
