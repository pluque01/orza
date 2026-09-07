package tui

import (
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
)

const convergenceTimeout = 3 * time.Second

type convergenceProgramModel struct {
	model    *Model
	init     tea.Cmd
	observed chan convergenceObservation
}

type convergenceObservation struct {
	helpModal         bool
	operation         bool
	securitySuspended bool
	securityActive    bool
}

func (m *convergenceProgramModel) Init() tea.Cmd { return m.init }

func (m *convergenceProgramModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, command := m.model.Update(msg)
	if m.observed != nil {
		m.observed <- convergenceObservation{
			helpModal:         m.model.modal.kind == modalKindHelp && m.model.focusOwner == focusOwnerModal,
			operation:         m.model.operation != nil,
			securitySuspended: m.model.securityInput != nil && m.model.securityInput.suspended,
			securityActive:    m.model.securityInput != nil,
		}
	}
	return m, command
}

func (m *convergenceProgramModel) View() tea.View { return m.model.View() }

func TestT089LiveSecretReaderCancelsOnUndersizedResize(t *testing.T) {
	for _, kind := range []app.SecretKind{app.SecretPassword, app.SecretPassphrase} {
		t.Run(string(kind), func(t *testing.T) {
			local := newBlockingConvergenceTerminal(terminal.Size{Columns: 80, Rows: 24})
			command := &sessionExecCommand{
				ctx: context.Background(), terminal: local, output: io.Discard,
				preservedFocus: focusOwnerDetail, width: 80, height: 24,
			}
			done := make(chan error, 1)
			go func() {
				_, err := command.readSecret(command.ctx, app.SecretRequest{Kind: kind})
				done <- err
			}()
			waitConvergence(t, local.started, "secret reader start")
			local.resize(terminal.Size{Columns: 39, Rows: 11})
			select {
			case err := <-done:
				if !errors.Is(err, context.Canceled) {
					t.Fatalf("resize error = %v, want canceled blocked reader", err)
				}
			case <-time.After(convergenceTimeout):
				local.cancel()
				<-done
				t.Fatal("live secret reader remained blocked after undersized resize")
			}
		})
	}
}

func TestT089TrustCannotAcceptLineBufferedAcrossUndersizedResize(t *testing.T) {
	local := newBlockingConvergenceTerminal(terminal.Size{Columns: 80, Rows: 24})
	input := newConvergenceLineReader()
	command := &sessionExecCommand{
		ctx: context.Background(), terminal: local, input: input, output: io.Discard, noColor: true,
		preservedFocus: focusOwnerDetail, width: 80, height: 24,
	}
	result := make(chan app.TrustDecision, 1)
	command.service = ConnectFunc(func(ctx context.Context, request app.ConnectRequest) (app.ConnectResult, error) {
		decision, err := request.DecideTrust(ctx, app.TrustDecisionPrompt{Status: app.HostTrustUnknown})
		if err != nil {
			return app.ConnectResult{}, err
		}
		result <- decision
		return app.ConnectResult{}, nil
	})
	done := make(chan error, 1)
	go func() { done <- command.Run() }()
	waitConvergence(t, input.started, "trust reader start")
	local.resize(terminal.Size{Columns: 39, Rows: 11})
	deadline := time.Now().Add(convergenceTimeout)
	for !command.securityInputSnapshot().suspended && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !command.securityInputSnapshot().suspended {
		t.Fatal("trust reader did not observe undersized transition")
	}
	input.line <- "once\n"
	local.resize(terminal.Size{Columns: 80, Rows: 24})
	waitConvergence(t, input.started, "fresh trust reader after resize")
	input.line <- "\n"
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if decision := <-result; decision != app.TrustReject {
		t.Fatalf("decision = %q, buffered undersized input was accepted", decision)
	}
}

func TestT090ProgramCancelReachesBlockedSSHStartAndJoins(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	service := ConnectFunc(func(ctx context.Context, _ app.ConnectRequest) (app.ConnectResult, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return app.ConnectResult{}, ctx.Err()
	})
	model := New(Config{Connect: service, Terminal: terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}), Width: 80, Height: 24, NoColor: true})
	command := model.sessionCommand(testConnection("server", syntheticRootID, "/server", 1))
	runConvergenceProgramCancel(t, model, command, started, stopped)
}

func TestT090ProgramCancelReachesBlockedRememberPasswordAndJoins(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	service := ConnectionFuncs{CreateFunc: func(ctx context.Context, _ app.CreateConnectionRequest) (app.ConnectionResult, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return app.ConnectionResult{}, ctx.Err()
	}}
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueSecret([]byte("ephemeral"), nil)
	model := New(Config{Connections: service, Terminal: local, Width: 80, Height: 24, NoColor: true})
	command := model.createCommand(app.CreateConnectionRequest{}, true, nil)
	runConvergenceProgramCancel(t, model, command, started, stopped)
}

func TestT090ProgramQuitWaitsForBlockedSSHStartCleanup(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	service := ConnectFunc(func(ctx context.Context, _ app.ConnectRequest) (app.ConnectResult, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return app.ConnectResult{}, ctx.Err()
	})
	model := New(Config{Connect: service, Terminal: terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}), Width: 80, Height: 24, NoColor: true})
	runConvergenceProgramQuit(t, model, model.sessionCommand(testConnection("server", syntheticRootID, "/server", 1)), started, stopped)
}

func TestT090ProgramQuitWaitsForRememberPasswordCleanup(t *testing.T) {
	started, stopped := make(chan struct{}), make(chan struct{})
	service := ConnectionFuncs{CreateFunc: func(ctx context.Context, _ app.CreateConnectionRequest) (app.ConnectionResult, error) {
		close(started)
		<-ctx.Done()
		close(stopped)
		return app.ConnectionResult{}, ctx.Err()
	}}
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	local.QueueSecret([]byte("ephemeral"), nil)
	model := New(Config{Connections: service, Terminal: local, Width: 80, Height: 24, NoColor: true})
	runConvergenceProgramQuit(t, model, model.createCommand(app.CreateConnectionRequest{}, true, nil), started, stopped)
}

func TestT090SessionActivationReleasesInputThenJoinsBeforeExit(t *testing.T) {
	activationReturned := make(chan struct{})
	release := make(chan struct{})
	cleaned := make(chan struct{})
	service := ConnectFunc(func(ctx context.Context, request app.ConnectRequest) (app.ConnectResult, error) {
		if err := request.Activate(ctx); err != nil {
			return app.ConnectResult{}, err
		}
		close(activationReturned)
		select {
		case <-ctx.Done():
			return app.ConnectResult{}, ctx.Err()
		case <-release:
		}
		close(cleaned)
		return app.ConnectResult{Session: app.SSHSessionResult{State: app.SessionSucceeded, Outcome: app.SessionOutcomeSuccess, StartedAt: time.Now()}}, nil
	})
	model := New(Config{Connect: service, Terminal: terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}), Width: 80, Height: 24, NoColor: true})
	model.focusOwner = focusOwnerDetail
	program := tea.NewProgram(
		&convergenceProgramModel{model: model, init: model.sessionCommand(testConnection("server", syntheticRootID, "/server", 1))},
		tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithWindowSize(80, 24),
	)
	runDone := make(chan error, 1)
	go func() {
		_, err := program.Run()
		runDone <- err
	}()
	waitConvergence(t, activationReturned, "session activation handoff")
	close(release)
	waitConvergence(t, cleaned, "session cleanup")
	select {
	case err := <-runDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(convergenceTimeout):
		program.Kill()
		<-runDone
		t.Fatal("program did not wait for and join the active session")
	}
	if model.focusOwner != focusOwnerDetail {
		t.Fatalf("session handoff changed focus to %v", model.focusOwner)
	}
}

func TestT089RememberPasswordResizeDiscardsBufferedSecret(t *testing.T) {
	local := newAttemptConvergenceTerminal(terminal.Size{Columns: 80, Rows: 24}, 3)
	saved := make(chan bool, 1)
	service := ConnectionFuncs{CreateFunc: func(_ context.Context, request app.CreateConnectionRequest) (app.ConnectionResult, error) {
		saved <- string(request.Password) == "fresh-after-resize"
		return app.ConnectionResult{}, nil
	}}
	model := New(Config{Connections: service, Terminal: local, Width: 80, Height: 24, NoColor: true})
	command := model.createCommand(app.CreateConnectionRequest{}, true, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := make(chan convergenceObservation, 16)
	program := tea.NewProgram(
		&convergenceProgramModel{model: model, init: command, observed: observed},
		tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithWindowSize(80, 24),
	)
	runDone := make(chan error, 1)
	go func() {
		_, err := program.Run()
		runDone <- err
	}()
	if attempt := local.waitAttempt(t); attempt != 0 {
		t.Fatalf("first secret attempt = %d", attempt)
	}
	local.resize(terminal.Size{Columns: 39, Rows: 11})
	if attempt := local.waitCanceled(t); attempt != 0 {
		t.Fatalf("canceled secret attempt = %d", attempt)
	}
	waitConvergenceObservation(t, observed, func(value convergenceObservation) bool { return value.securitySuspended }, "remember-password suspension")
	local.setSize(terminal.Size{Columns: 80, Rows: 24})
	program.Send(tea.WindowSizeMsg{Width: 80, Height: 24})
	if attempt := local.waitAttempt(t); attempt != 1 {
		t.Fatalf("discard attempt = %d", attempt)
	}
	local.respond(1, []byte("buffered-while-undersized"))
	if attempt := local.waitAttempt(t); attempt != 2 {
		t.Fatalf("fresh attempt = %d", attempt)
	}
	local.respond(2, []byte("fresh-after-resize"))
	select {
	case accepted := <-saved:
		if !accepted {
			t.Fatal("remembered-password persistence accepted buffered or changed input")
		}
	case <-time.After(convergenceTimeout):
		t.Fatal("remembered-password persistence did not resume")
	}
	program.Quit()
	<-runDone
}

func runConvergenceProgramCancel(t *testing.T, model *Model, command tea.Cmd, started, stopped <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	observed := make(chan convergenceObservation, 16)
	program := tea.NewProgram(
		&convergenceProgramModel{model: model, init: command, observed: observed},
		tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithWindowSize(80, 24),
	)
	runDone := make(chan error, 1)
	go func() {
		_, err := program.Run()
		runDone <- err
	}()
	waitConvergence(t, started, "operation start")
	go program.Send(keyPress("?"))
	waitConvergenceObservation(t, observed, func(value convergenceObservation) bool { return value.helpModal && value.operation }, "operation Help modal")
	go program.Send(keyPress("esc"))
	waitConvergenceObservation(t, observed, func(value convergenceObservation) bool { return !value.helpModal && value.operation }, "Help close")
	go program.Send(keyPress("esc"))
	select {
	case <-stopped:
		waitConvergenceObservation(t, observed, func(value convergenceObservation) bool { return !value.operation }, "operation cleanup")
		program.Quit()
		<-runDone
	case <-time.After(convergenceTimeout):
		cancel()
		<-runDone
		t.Fatal("Esc did not cancel and join the real operation")
	}
}

func runConvergenceProgramQuit(t *testing.T, model *Model, command tea.Cmd, started, stopped <-chan struct{}) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	program := tea.NewProgram(
		&convergenceProgramModel{model: model, init: command},
		tea.WithContext(ctx), tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithWindowSize(80, 24),
	)
	runDone := make(chan error, 1)
	go func() {
		_, err := program.Run()
		runDone <- err
	}()
	waitConvergence(t, started, "operation start")
	go program.Send(tea.KeyPressMsg(tea.Key{Code: 'c', Mod: tea.ModCtrl}))
	select {
	case <-stopped:
	case <-time.After(convergenceTimeout):
		cancel()
		<-runDone
		t.Fatal("Quit did not cancel and join the real operation")
	}
	select {
	case <-runDone:
	case <-time.After(convergenceTimeout):
		cancel()
		<-runDone
		t.Fatal("program did not exit after operation cleanup")
	}
}

func waitConvergenceObservation(t *testing.T, observed <-chan convergenceObservation, accept func(convergenceObservation) bool, operation string) {
	t.Helper()
	timer := time.NewTimer(convergenceTimeout)
	defer timer.Stop()
	var last convergenceObservation
	for {
		select {
		case value := <-observed:
			last = value
			if accept(value) {
				return
			}
		case <-timer.C:
			t.Fatalf("timed out waiting for %s; last observation: %+v", operation, last)
		}
	}
}

func waitConvergence(t *testing.T, signal <-chan struct{}, operation string) {
	t.Helper()
	select {
	case <-signal:
	case <-time.After(convergenceTimeout):
		t.Fatalf("timed out waiting for %s", operation)
	}
}

type attemptConvergenceTerminal struct {
	mu        sync.Mutex
	size      terminal.Size
	started   chan int
	canceled  chan int
	responses []chan []byte
	next      int
	watchers  map[chan terminal.Size]struct{}
}

func newAttemptConvergenceTerminal(size terminal.Size, attempts int) *attemptConvergenceTerminal {
	responses := make([]chan []byte, attempts)
	for index := range responses {
		responses[index] = make(chan []byte, 1)
	}
	return &attemptConvergenceTerminal{
		size: size, started: make(chan int, attempts), canceled: make(chan int, attempts), responses: responses,
		watchers: make(map[chan terminal.Size]struct{}),
	}
}

func (t *attemptConvergenceTerminal) Interactive() bool { return true }
func (t *attemptConvergenceTerminal) Capture(context.Context) (terminal.State, error) {
	return terminal.State{}, nil
}
func (t *attemptConvergenceTerminal) VTCapability(context.Context) (terminal.VTCapability, error) {
	return terminal.VTCapability{Status: terminal.VTSupported}, nil
}
func (t *attemptConvergenceTerminal) Size(context.Context) (terminal.Size, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.size, nil
}
func (t *attemptConvergenceTerminal) MakeRaw(context.Context) error                 { return nil }
func (t *attemptConvergenceTerminal) Restore(context.Context, terminal.State) error { return nil }
func (t *attemptConvergenceTerminal) ReadSecret(ctx context.Context, _ terminal.SecretPrompt) ([]byte, error) {
	t.mu.Lock()
	attempt := t.next
	var response chan []byte
	if attempt < len(t.responses) {
		response = t.responses[attempt]
		t.next++
	}
	t.mu.Unlock()
	if response == nil {
		return nil, terminal.ErrNoSecretResponse
	}
	t.started <- attempt
	select {
	case <-ctx.Done():
		t.canceled <- attempt
		return nil, ctx.Err()
	case secret := <-response:
		return append([]byte(nil), secret...), nil
	}
}
func (t *attemptConvergenceTerminal) ResizeEvents(ctx context.Context) (<-chan terminal.Size, error) {
	events := make(chan terminal.Size, 4)
	t.mu.Lock()
	t.watchers[events] = struct{}{}
	t.mu.Unlock()
	go func() {
		<-ctx.Done()
		t.mu.Lock()
		delete(t.watchers, events)
		close(events)
		t.mu.Unlock()
	}()
	return events, nil
}
func (t *attemptConvergenceTerminal) setSize(size terminal.Size) {
	t.mu.Lock()
	t.size = size
	t.mu.Unlock()
}
func (t *attemptConvergenceTerminal) resize(size terminal.Size) {
	t.mu.Lock()
	t.size = size
	for watcher := range t.watchers {
		watcher <- size
	}
	t.mu.Unlock()
}
func (t *attemptConvergenceTerminal) waitAttempt(testingT *testing.T) int {
	testingT.Helper()
	select {
	case attempt := <-t.started:
		return attempt
	case <-time.After(convergenceTimeout):
		testingT.Fatal("timed out waiting for secret attempt")
		return -1
	}
}
func (t *attemptConvergenceTerminal) waitCanceled(testingT *testing.T) int {
	testingT.Helper()
	select {
	case attempt := <-t.canceled:
		return attempt
	case <-time.After(convergenceTimeout):
		testingT.Fatal("timed out waiting for secret cancellation")
		return -1
	}
}
func (t *attemptConvergenceTerminal) respond(attempt int, secret []byte) {
	t.mu.Lock()
	response := t.responses[attempt]
	t.mu.Unlock()
	response <- secret
}

type blockingConvergenceTerminal struct {
	mu       sync.Mutex
	size     terminal.Size
	resizes  chan terminal.Size
	started  chan struct{}
	stop     context.CancelFunc
	stopOnce sync.Once
}

func newBlockingConvergenceTerminal(size terminal.Size) *blockingConvergenceTerminal {
	return &blockingConvergenceTerminal{size: size, resizes: make(chan terminal.Size, 4), started: make(chan struct{})}
}

func (t *blockingConvergenceTerminal) Interactive() bool { return true }
func (t *blockingConvergenceTerminal) Capture(context.Context) (terminal.State, error) {
	return terminal.State{}, nil
}
func (t *blockingConvergenceTerminal) VTCapability(context.Context) (terminal.VTCapability, error) {
	return terminal.VTCapability{Status: terminal.VTSupported}, nil
}
func (t *blockingConvergenceTerminal) Size(context.Context) (terminal.Size, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.size, nil
}
func (t *blockingConvergenceTerminal) MakeRaw(context.Context) error                 { return nil }
func (t *blockingConvergenceTerminal) Restore(context.Context, terminal.State) error { return nil }
func (t *blockingConvergenceTerminal) ReadSecret(ctx context.Context, _ terminal.SecretPrompt) ([]byte, error) {
	readCtx, cancel := context.WithCancel(ctx)
	t.mu.Lock()
	t.stop = cancel
	t.mu.Unlock()
	t.stopOnce.Do(func() { close(t.started) })
	<-readCtx.Done()
	return nil, readCtx.Err()
}
func (t *blockingConvergenceTerminal) ResizeEvents(ctx context.Context) (<-chan terminal.Size, error) {
	events := make(chan terminal.Size, 4)
	go func() {
		defer close(events)
		for {
			select {
			case <-ctx.Done():
				return
			case size := <-t.resizes:
				events <- size
			}
		}
	}()
	return events, nil
}
func (t *blockingConvergenceTerminal) resize(size terminal.Size) {
	t.mu.Lock()
	t.size = size
	t.mu.Unlock()
	t.resizes <- size
}
func (t *blockingConvergenceTerminal) cancel() {
	t.mu.Lock()
	cancel := t.stop
	t.mu.Unlock()
	if cancel != nil {
		cancel()
	}
}

type convergenceLineReader struct {
	started chan struct{}
	line    chan string
}

func newConvergenceLineReader() *convergenceLineReader {
	return &convergenceLineReader{started: make(chan struct{}, 4), line: make(chan string, 4)}
}

func (r *convergenceLineReader) Read(value []byte) (int, error) {
	r.started <- struct{}{}
	line := <-r.line
	return copy(value, line), nil
}
