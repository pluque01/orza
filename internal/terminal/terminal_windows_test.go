//go:build windows

package terminal

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/sys/windows"
	"golang.org/x/term"
)

const testWindowsOutputMode = uint32(0x20)
const testWindowsInputMode = uint32(0x02)

func TestWindowsSystemCaptureRawRestore(t *testing.T) {
	input, output := windowsTestFiles(t)
	system := New(input, output)
	native := new(term.State)
	var restored *term.State
	system.ops.getState = func(int) (*term.State, error) { return native, nil }
	system.ops.makeRaw = func(int) (*term.State, error) { return native, nil }
	system.ops.restore = func(_ int, state *term.State) error { restored = state; return nil }
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
	if restored != native || system.raw {
		t.Fatal("native console state was not restored")
	}
}

func TestWindowsVTCapabilityProbeRestoresConsoleModes(t *testing.T) {
	input, output := windowsTestFiles(t)
	system := New(input, output)
	system.ops.getInputMode = func(int) (uint32, error) { return testWindowsInputMode, nil }
	system.ops.getOutputMode = func(int) (uint32, error) { return testWindowsOutputMode, nil }
	var inputModes, outputModes []uint32
	system.ops.setInputMode = func(_ int, mode uint32) error {
		inputModes = append(inputModes, mode)
		return nil
	}
	system.ops.setOutputMode = func(_ int, mode uint32) error {
		outputModes = append(outputModes, mode)
		return nil
	}

	capability, err := system.VTCapability(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capability.Status != VTSupported {
		t.Fatalf("VTCapability() = %+v, want supported", capability)
	}
	wantInput := testWindowsInputMode | windows.ENABLE_VIRTUAL_TERMINAL_INPUT
	wantOutput := testWindowsOutputMode | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING | windows.DISABLE_NEWLINE_AUTO_RETURN
	if len(inputModes) != 2 || inputModes[0] != wantInput || inputModes[1] != testWindowsInputMode {
		t.Fatalf("input mode lifecycle = %v, want [%d %d]", inputModes, wantInput, testWindowsInputMode)
	}
	if len(outputModes) != 2 || outputModes[0] != wantOutput || outputModes[1] != testWindowsOutputMode {
		t.Fatalf("output mode lifecycle = %v, want [%d %d]", outputModes, wantOutput, testWindowsOutputMode)
	}
}

func TestWindowsVTCapabilityMissingRestoresChangedInputMode(t *testing.T) {
	input, output := windowsTestFiles(t)
	system := New(input, output)
	system.ops.getInputMode = func(int) (uint32, error) { return testWindowsInputMode, nil }
	system.ops.getOutputMode = func(int) (uint32, error) { return testWindowsOutputMode, nil }
	var inputModes []uint32
	system.ops.setInputMode = func(_ int, mode uint32) error {
		inputModes = append(inputModes, mode)
		return nil
	}
	system.ops.setOutputMode = func(int, uint32) error { return errors.New("VT output is unavailable") }

	capability, err := system.VTCapability(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if capability != (VTCapability{Status: VTMissing}) {
		t.Fatalf("VTCapability() = %+v, want missing without fallback", capability)
	}
	if len(inputModes) != 2 || inputModes[1] != testWindowsInputMode {
		t.Fatalf("input mode lifecycle = %v, want enable followed by restoration", inputModes)
	}
}

func TestWindowsSystemPollsChangedSizesAndStops(t *testing.T) {
	input, output := windowsTestFiles(t)
	system := New(input, output)
	system.pollInterval = time.Millisecond
	system.ops.isTerminal = func(int) bool { return true }
	var mu sync.Mutex
	sizes := []Size{{Columns: 80, Rows: 24}, {Columns: 80, Rows: 24}, {Columns: 120, Rows: 40}}
	system.ops.getSize = func(int) (int, int, error) {
		mu.Lock()
		defer mu.Unlock()
		size := sizes[0]
		if len(sizes) > 1 {
			sizes = sizes[1:]
		}
		return size.Columns, size.Rows, nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	events, err := system.ResizeEvents(ctx)
	if err != nil {
		t.Fatal(err)
	}
	select {
	case size := <-events:
		if size != (Size{Columns: 120, Rows: 40}) {
			t.Fatalf("size = %v", size)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for polled resize")
	}
	cancel()
	select {
	case _, ok := <-events:
		if ok {
			t.Fatal("resize stream remains open")
		}
	case <-time.After(time.Second):
		t.Fatal("resize stream did not stop")
	}
}

func TestWindowsReadSecretStandardConsoleLifecycle(t *testing.T) {
	input, output := windowsTestFiles(t)
	system := New(input, output)
	native := new(term.State)
	system.ops.isTerminal = func(int) bool { return true }
	system.ops.standardInput = func(*os.File) bool { return true }
	system.ops.makeRaw = func(int) (*term.State, error) { return native, nil }
	system.ops.getOutputMode = func(int) (uint32, error) { return testWindowsOutputMode, nil }
	var outputModes []uint32
	system.ops.setOutputMode = func(_ int, mode uint32) error {
		outputModes = append(outputModes, mode)
		return nil
	}
	order := new(callOrder)
	system.ops.secretReader = func(io.Reader) (secretReaderPair, error) {
		return testSecretReaderPair(
			&fakeSecretCancelReader{order: order},
			&fakeSecretStreamer{order: order, events: []uv.Event{
				uv.PasteEvent{Content: "windows-canary"},
				uv.KeyPressEvent(uv.Key{Code: uv.KeyEnter}),
			}},
		), nil
	}

	secret, err := system.ReadSecret(context.Background(), SecretPrompt{Message: "Password: "})
	if err != nil {
		t.Fatal(err)
	}
	if string(secret) != "windows-canary" {
		t.Fatal("ReadSecret did not return submitted input")
	}
	wipe(secret)
	contents, err := os.ReadFile(output.Name())
	if err != nil {
		t.Fatal(err)
	}
	printed := string(contents)
	if strings.Contains(printed, "windows-canary") || !strings.Contains(printed, ansi.SetModeBracketedPaste) ||
		!strings.Contains(printed, ansi.ResetModeBracketedPaste) || !strings.HasSuffix(printed, "\r\n") {
		t.Fatal("Windows prompt leaked input or omitted VT/CRLF cleanup")
	}
	if system.raw {
		t.Fatal("Windows console mode was not restored")
	}
	wantVTMode := testWindowsOutputMode | windows.ENABLE_PROCESSED_OUTPUT | windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	if len(outputModes) != 2 || outputModes[0] != wantVTMode || outputModes[1] != testWindowsOutputMode {
		t.Fatalf("output mode lifecycle = %v, want [%d %d]", outputModes, wantVTMode, testWindowsOutputMode)
	}
}

func TestWindowsReadSecretRejectsFallbackBeforeCaptureOrRaw(t *testing.T) {
	if os.Stdin == nil {
		t.Skip("process standard input is unavailable")
	}
	_, output := windowsTestFiles(t)
	system := New(os.Stdin, output)
	system.ops.isTerminal = func(int) bool { return true }
	system.ops.standardInput = func(*os.File) bool { return true }
	order := new(callOrder)
	system.ops.secretReader = func(input io.Reader) (secretReaderPair, error) {
		return newWindowsUVSecretReader(input, func(io.Reader) (secretCancelReader, bool, error) {
			return &fakeSecretCancelReader{order: order}, false, nil
		})
	}
	rawCalled := false
	captureCalled := false
	system.ops.makeRaw = func(int) (*term.State, error) {
		rawCalled = true
		return nil, nil
	}
	system.ops.getState = func(int) (*term.State, error) {
		captureCalled = true
		return new(term.State), nil
	}

	secret, err := system.ReadSecret(context.Background(), SecretPrompt{Message: "Password: "})
	if err == nil || secret != nil || rawCalled || captureCalled {
		t.Fatal("fallback Windows reader did not fail closed before capture or raw mode")
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
		t.Fatal("uncancelable Windows reader wrote a prompt or terminal sequence")
	}
}

func TestWindowsReadSecretVTEnableFailureRestoresBeforeWriting(t *testing.T) {
	input, output := windowsTestFiles(t)
	system := New(input, output)
	native := new(term.State)
	readerCalled := false
	streamStarted := make(chan struct{})
	order := new(callOrder)
	system.ops.isTerminal = func(int) bool { return true }
	system.ops.standardInput = func(*os.File) bool { return true }
	system.ops.makeRaw = func(int) (*term.State, error) { return native, nil }
	system.ops.getOutputMode = func(int) (uint32, error) { return testWindowsOutputMode, nil }
	var outputModes []uint32
	system.ops.setOutputMode = func(_ int, mode uint32) error {
		outputModes = append(outputModes, mode)
		if len(outputModes) == 1 {
			return errors.New("injected VT failure")
		}
		return nil
	}
	system.ops.secretReader = func(io.Reader) (secretReaderPair, error) {
		readerCalled = true
		return testSecretReaderPair(
			&fakeSecretCancelReader{order: order},
			&fakeSecretStreamer{order: order, started: streamStarted},
		), nil
	}

	secret, err := system.ReadSecret(context.Background(), SecretPrompt{Message: "Password: "})
	if err == nil || secret != nil || !readerCalled || system.raw {
		t.Fatal("VT mode failure did not fail closed and restore input")
	}
	select {
	case <-streamStarted:
		t.Fatal("input stream started after VT mode enablement failed")
	default:
	}
	if len(outputModes) != 2 || outputModes[1] != testWindowsOutputMode {
		t.Fatalf("output modes = %v, want failed enable followed by restoration", outputModes)
	}
	contents, readErr := os.ReadFile(output.Name())
	if readErr != nil {
		t.Fatal(readErr)
	}
	if len(contents) != 0 {
		t.Fatal("VT sequences were written after output mode enablement failed")
	}
}

func TestWindowsReadSecretAlternateHandleFailsClosedBeforeRaw(t *testing.T) {
	input, output := windowsTestFiles(t)
	system := New(input, output)
	system.ops.isTerminal = func(int) bool { return true }
	system.ops.standardInput = func(*os.File) bool { return false }
	rawCalled := false
	readerCalled := false
	system.ops.makeRaw = func(int) (*term.State, error) { rawCalled = true; return nil, nil }
	system.ops.secretReader = func(io.Reader) (secretReaderPair, error) {
		readerCalled = true
		return secretReaderPair{}, nil
	}

	secret, err := system.ReadSecret(context.Background(), SecretPrompt{Message: "Password: "})
	if err == nil || secret != nil || rawCalled || readerCalled {
		t.Fatal("alternate console input did not fail closed before reader setup or raw mode")
	}
	if !strings.Contains(err.Error(), "standard console input handle") || strings.Contains(err.Error(), "Password") {
		t.Fatal("alternate-handle error was not actionable and secret-free")
	}
}

func TestWindowsUVSecretReaderFactoryRequiresProcessStdin(t *testing.T) {
	if os.Stdin == nil {
		t.Skip("process standard input is unavailable")
	}
	order := new(callOrder)
	calledWithStdin := false
	pair, err := newWindowsUVSecretReader(os.Stdin, func(input io.Reader) (secretCancelReader, bool, error) {
		calledWithStdin = input == os.Stdin
		return &fakeSecretCancelReader{order: order}, true, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if !calledWithStdin || !pair.cancellationGuaranteed || pair.streamer == nil {
		t.Fatal("process stdin did not select the certified Ultraviolet reader path")
	}
	if err := closeIdleSecretReader(pair); err != nil {
		t.Fatal(err)
	}

	input, _ := windowsTestFiles(t)
	alternateFactoryCalled := false
	pair, err = newWindowsUVSecretReader(input, func(io.Reader) (secretCancelReader, bool, error) {
		alternateFactoryCalled = true
		return &fakeSecretCancelReader{order: order}, true, nil
	})
	if !errors.Is(err, errSecretReaderUnsafe) || pair.cancelReader != nil || alternateFactoryCalled {
		t.Fatal("alternate input reached the Ultraviolet reader factory")
	}
}

func TestWindowsReadSecretContextCancellationRestores(t *testing.T) {
	input, output := windowsTestFiles(t)
	system := New(input, output)
	restoredOutput := make(chan struct{}, 1)
	system.ops.isTerminal = func(int) bool { return true }
	system.ops.standardInput = func(*os.File) bool { return true }
	system.ops.makeRaw = func(int) (*term.State, error) { return new(term.State), nil }
	system.ops.getOutputMode = func(int) (uint32, error) { return testWindowsOutputMode, nil }
	system.ops.setOutputMode = func(_ int, mode uint32) error {
		if mode == testWindowsOutputMode {
			restoredOutput <- struct{}{}
		}
		return nil
	}
	started := make(chan struct{})
	order := new(callOrder)
	system.ops.secretReader = func(io.Reader) (secretReaderPair, error) {
		return testSecretReaderPair(&fakeSecretCancelReader{order: order}, &fakeSecretStreamer{order: order, started: started}), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan error, 1)
	go func() { _, err := system.ReadSecret(ctx, SecretPrompt{Message: "Password: "}); result <- err }()
	<-started
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("context cancellation was not preserved")
		}
	case <-time.After(time.Second):
		t.Fatal("Windows secret reader did not shut down")
	}
	select {
	case <-restoredOutput:
	default:
		t.Fatal("Windows console output mode was not restored after cancellation")
	}
}

func windowsTestFiles(t *testing.T) (*os.File, *os.File) {
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
