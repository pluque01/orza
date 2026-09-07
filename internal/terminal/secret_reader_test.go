package terminal

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
)

func TestSecretReaderPromptOutcomeMatrixAndConfidentiality(t *testing.T) {
	prompts := []string{"Remember password: ", "Password: ", "Private key passphrase: "}
	outcomes := []struct {
		name         string
		events       []uv.Event
		readErr      error
		wantOK       bool
		wantCanceled bool
	}{
		{name: "submit", events: []uv.Event{uv.PasteEvent{Content: "SECRET_CANARY_002"}, uv.KeyPressEvent(uv.Key{Code: uv.KeyEnter})}, wantOK: true},
		{name: "cancel", events: []uv.Event{uv.PasteEvent{Content: "SECRET_CANARY_002"}, uv.KeyPressEvent(uv.Key{Code: uv.KeyEscape})}, wantCanceled: true},
		{name: "input-failure", events: []uv.Event{uv.PasteEvent{Content: "SECRET_CANARY_002"}}, readErr: errors.New("injected input failure")},
	}

	for promptIndex, prompt := range prompts {
		for _, outcome := range outcomes {
			t.Run(fmt.Sprintf("prompt-%d/%s", promptIndex, outcome.name), func(t *testing.T) {
				order := new(callOrder)
				reader := &fakeSecretCancelReader{order: order}
				streamer := &fakeSecretStreamer{events: outcome.events, err: outcome.readErr, order: order}
				output := &trackedSecretOutput{order: order}
				secret, err := runSecretPrompt(context.Background(), SecretPrompt{Message: prompt}, output, "\n", func(context.Context) error {
					order.add("restore")
					return nil
				}, testSecretReaderPair(reader, streamer))
				if outcome.wantOK {
					if err != nil || len(secret) != len("SECRET_CANARY_002") {
						t.Fatal("submitted secret was not returned after cleanup")
					}
					wipe(secret)
				} else if err == nil || secret != nil {
					t.Fatal("failed/canceled prompt returned a secret")
				}
				if outcome.wantCanceled && !errors.Is(err, context.Canceled) {
					t.Fatal("physical cancellation did not return context cancellation")
				}
				assertNoSecretSurface(t, output.String(), err)
				assertCleanupOrder(t, order.snapshot())
				if contents := output.String(); !strings.Contains(contents, ansi.SetModeBracketedPaste) ||
					!strings.Contains(contents, ansi.ResetModeBracketedPaste) || !strings.HasSuffix(contents, "\n") {
					t.Fatal("prompt output did not contain the bracketed-paste lifecycle and newline")
				}
			})
		}
	}
}

func TestSecretReaderContextCancellationJoinsAndRestores(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	order := new(callOrder)
	reader := &fakeSecretCancelReader{order: order}
	streamer := &fakeSecretStreamer{started: started, order: order}
	output := &trackedSecretOutput{order: order}
	result := make(chan error, 1)
	go func() {
		_, err := runSecretPrompt(ctx, SecretPrompt{Message: "Password: "}, output, "\n", func(context.Context) error {
			order.add("restore")
			return nil
		}, testSecretReaderPair(reader, streamer))
		result <- err
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("reader did not start")
	}
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatal("context cancellation was not preserved")
		}
	case <-time.After(time.Second):
		t.Fatal("canceled reader was not joined")
	}
	assertCleanupOrder(t, order.snapshot())
}

func TestSecretReaderCancelFailureDoesNotRestoreBeforeBlockedStreamExits(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{})
	release := make(chan struct{})
	exited := make(chan struct{})
	cancelCalled := make(chan struct{})
	order := new(callOrder)
	reader := &fakeSecretCancelReader{order: order, cancelFails: true, cancelCalled: cancelCalled}
	streamer := &fakeSecretStreamer{started: started, blocked: release, exited: exited, order: order}
	output := &trackedSecretOutput{order: order}
	result := make(chan error, 1)
	go func() {
		_, err := runSecretPrompt(ctx, SecretPrompt{Message: "Password: "}, output, "\n", func(context.Context) error {
			order.add("restore")
			return nil
		}, testSecretReaderPair(reader, streamer))
		result <- err
	}()

	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("reader did not start")
	}
	cancel()
	select {
	case <-cancelCalled:
	case <-time.After(time.Second):
		t.Fatal("reader cancellation was not attempted")
	}
	select {
	case err := <-result:
		t.Fatalf("prompt returned before stream exit: %v", err)
	case <-time.After(50 * time.Millisecond):
	}
	if calls := order.snapshot(); containsCall(calls, "restore") || containsCall(calls, "close") {
		t.Fatalf("reader was closed or terminal restored before stream exit: %v", calls)
	}
	close(release)
	select {
	case <-exited:
	case <-time.After(time.Second):
		t.Fatal("released stream goroutine did not exit")
	}
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) || !errors.Is(err, errSecretReaderCancel) {
			t.Fatalf("cleanup error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("prompt did not return after stream exit")
	}
	assertCleanupOrder(t, order.snapshot())
}

func TestSecretReaderCleanupFailuresWipeSubmittedResult(t *testing.T) {
	failures := []struct {
		name       string
		closeErr   error
		restoreErr error
	}{
		{name: "reader-close", closeErr: errors.New("close canary")},
		{name: "restore", restoreErr: errors.New("restore canary")},
	}
	for _, failure := range failures {
		t.Run(failure.name, func(t *testing.T) {
			reader := &fakeSecretCancelReader{closeErr: failure.closeErr, order: new(callOrder)}
			streamer := &fakeSecretStreamer{events: []uv.Event{uv.PasteEvent{Content: "SECRET_CANARY_002"}, uv.KeyPressEvent(uv.Key{Code: uv.KeyEnter})}, order: reader.order}
			var output bytes.Buffer
			secret, err := runSecretPrompt(context.Background(), SecretPrompt{Message: "Password: "}, &output, "\n", func(context.Context) error {
				reader.order.add("restore")
				return failure.restoreErr
			}, testSecretReaderPair(reader, streamer))
			if err == nil || secret != nil {
				t.Fatal("cleanup failure returned submitted secret")
			}
			assertNoSecretSurface(t, output.String(), err)
		})
	}
}

func TestSecretReaderOutputFailureStillRestores(t *testing.T) {
	order := new(callOrder)
	output := &failingSecretOutput{writesLeft: 1}
	reader := &fakeSecretCancelReader{order: order}
	streamer := &fakeSecretStreamer{order: order}
	secret, err := runSecretPrompt(context.Background(), SecretPrompt{Message: "Password: "}, output, "\n", func(context.Context) error {
		order.add("restore")
		return nil
	}, testSecretReaderPair(reader, streamer))
	if err == nil || secret != nil {
		t.Fatal("output failure returned a secret")
	}
	if got := order.snapshot(); len(got) == 0 || got[len(got)-1] != "restore" {
		t.Fatal("terminal was not restored after output failure")
	}
}

func assertNoSecretSurface(t *testing.T, output string, err error) {
	t.Helper()
	for _, surface := range []string{output, fmt.Sprint(err), fmt.Sprintf("%#v", err)} {
		if strings.Contains(surface, "SECRET_CANARY_002") {
			t.Fatal("secret appeared in a confidentiality surface")
		}
	}
}

func assertCleanupOrder(t *testing.T, order []string) {
	t.Helper()
	positions := make(map[string]int)
	for index, call := range order {
		positions[call] = index
	}
	for _, call := range []string{"cancel", "join", "close", "paste-off", "cursor-newline", "restore"} {
		if _, ok := positions[call]; !ok {
			t.Fatalf("cleanup did not perform %s", call)
		}
	}
	if !(positions["cancel"] < positions["close"] && positions["join"] < positions["close"] &&
		positions["close"] < positions["paste-off"] && positions["paste-off"] < positions["cursor-newline"] &&
		positions["cursor-newline"] < positions["restore"]) {
		t.Fatalf("cleanup order = %v", order)
	}
}

func containsCall(calls []string, want string) bool {
	for _, call := range calls {
		if call == want {
			return true
		}
	}
	return false
}

func testSecretReaderPair(reader secretCancelReader, streamer secretEventStreamer) secretReaderPair {
	return secretReaderPair{cancelReader: reader, streamer: streamer, cancellationGuaranteed: true}
}

type callOrder struct {
	mu    sync.Mutex
	calls []string
}

func (o *callOrder) add(call string) {
	o.mu.Lock()
	o.calls = append(o.calls, call)
	o.mu.Unlock()
}

func (o *callOrder) snapshot() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return append([]string(nil), o.calls...)
}

type fakeSecretCancelReader struct {
	order        *callOrder
	closeErr     error
	cancelFails  bool
	cancelCalled chan<- struct{}
}

func (r *fakeSecretCancelReader) Read([]byte) (int, error) { return 0, io.EOF }
func (r *fakeSecretCancelReader) Cancel() bool {
	r.order.add("cancel")
	if r.cancelCalled != nil {
		r.cancelCalled <- struct{}{}
	}
	return !r.cancelFails
}
func (r *fakeSecretCancelReader) Close() error { r.order.add("close"); return r.closeErr }

type fakeSecretStreamer struct {
	events  []uv.Event
	err     error
	started chan struct{}
	blocked <-chan struct{}
	exited  chan<- struct{}
	order   *callOrder
}

func (s *fakeSecretStreamer) StreamEvents(ctx context.Context, events chan<- uv.Event) error {
	if s.started != nil {
		close(s.started)
	}
	if s.blocked != nil {
		<-s.blocked
		s.order.add("join")
		if s.exited != nil {
			close(s.exited)
		}
		return nil
	}
	for _, event := range s.events {
		select {
		case events <- event:
		case <-ctx.Done():
			s.order.add("join")
			return nil
		}
	}
	if s.err != nil {
		s.order.add("join")
		return s.err
	}
	<-ctx.Done()
	s.order.add("join")
	return nil
}

type trackedSecretOutput struct {
	bytes.Buffer
	order *callOrder
}

func (o *trackedSecretOutput) Write(value []byte) (int, error) {
	o.record(string(value))
	return o.Buffer.Write(value)
}

func (o *trackedSecretOutput) WriteString(value string) (int, error) {
	o.record(value)
	return o.Buffer.WriteString(value)
}

func (o *trackedSecretOutput) record(text string) {
	if strings.Contains(text, ansi.ResetModeBracketedPaste) {
		o.order.add("paste-off")
	}
	if strings.Contains(text, ansi.ResetStyle) && strings.Contains(text, ansi.ShowCursor) {
		o.order.add("cursor-newline")
	}
}

type failingSecretOutput struct{ writesLeft int }

func (o *failingSecretOutput) Write(value []byte) (int, error) {
	if o.writesLeft == 0 {
		return 0, errors.New("injected output failure")
	}
	o.writesLeft--
	return len(value), nil
}
