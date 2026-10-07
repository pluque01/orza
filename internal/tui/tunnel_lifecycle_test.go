package tui

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
)

type fakeTunnels struct {
	mu           sync.Mutex
	entries      []app.TunnelSnapshot
	request      app.TunnelRequest
	closed       bool
	stops        []uint64
	stopAttempts [][2]uint64
}

func (f *fakeTunnels) Start(_ context.Context, r app.TunnelRequest) (app.TunnelSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.request = r
	s := app.TunnelSnapshot{ID: uint64(len(f.entries) + 1), Attempt: 1, Version: 1, State: app.TunnelStarting, Config: r.Config, Connection: app.SSHAttemptTarget{ID: r.Connection.ID, Revision: *r.Expected, Path: "/captured", Host: "host.test", Port: 22}}
	f.entries = append(f.entries, s)
	return s, nil
}
func (f *fakeTunnels) Retry(ctx context.Context, id uint64, r app.TunnelRequest) (app.TunnelSnapshot, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, s := range f.entries {
		if s.ID == id && !s.Live() {
			f.request = r
			s.Attempt++
			s.State = app.TunnelStarting
			s.Config = r.Config
			f.entries[i] = s
			return s, nil
		}
	}
	return app.TunnelSnapshot{}, app.ErrConflict
}
func (f *fakeTunnels) Stop(_ context.Context, id uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops = append(f.stops, id)
	for i := range f.entries {
		if f.entries[i].ID == id {
			f.entries[i].State = app.TunnelStopped
		}
	}
	return nil
}
func (f *fakeTunnels) StopAttempt(_ context.Context, id, attempt uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stopAttempts = append(f.stopAttempts, [2]uint64{id, attempt})
	for i := range f.entries {
		if f.entries[i].ID == id && f.entries[i].Attempt == attempt {
			f.stops = append(f.stops, id)
			f.entries[i].State = app.TunnelStopped
			return nil
		}
	}
	return nil
}
func (f *fakeTunnels) Snapshots() []app.TunnelSnapshot {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]app.TunnelSnapshot(nil), f.entries...)
}
func (f *fakeTunnels) Get(id uint64) (app.TunnelSnapshot, bool) {
	for _, s := range f.Snapshots() {
		if s.ID == id {
			return s, true
		}
	}
	return app.TunnelSnapshot{}, false
}
func (f *fakeTunnels) Dismiss(id uint64) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, s := range f.entries {
		if s.ID == id {
			if s.Live() {
				return app.ErrConflict
			}
			f.entries = append(f.entries[:i], f.entries[i+1:]...)
			return nil
		}
	}
	return app.ErrNotFound
}
func (f *fakeTunnels) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	for i := range f.entries {
		f.entries[i].State = app.TunnelStopped
	}
	return nil
}
func (f *fakeTunnels) Wait(context.Context, uint64) error { return nil }

var _ TunnelService = (*fakeTunnels)(nil)
var _ TunnelService = (*app.TunnelService)(nil)

func TestTunnelModeDraftConsentAndValidation(t *testing.T) {
	f := newTunnelForm(testConnection("host", syntheticRootID, "/host", 3), app.TunnelConfig{}, 0)
	if f.mode != app.TunnelLocal || f.inputs[0].Value() != "127.0.0.1" {
		t.Fatal("unsafe defaults")
	}
	f.inputs[1].SetValue("8080")
	f.inputs[2].SetValue("db.internal")
	f.inputs[3].SetValue("5432")
	f.inputs[0].SetValue("0.0.0.0")
	if _, ok := f.config(); ok {
		t.Fatal("exposure implicitly acknowledged")
	}
	f.ack = true
	if _, ok := f.config(); !ok {
		t.Fatal(f.error)
	}
	f.focus = 0
	f.update(keyPress("right"))
	if f.mode != app.TunnelRemote || f.ack {
		t.Fatal("mode did not clear consent")
	}
	f.update(keyPress("right"))
	if f.mode != app.TunnelDynamic {
		t.Fatal("dynamic unavailable")
	}
	f.inputs[0].SetValue("::1")
	f.inputs[3].SetValue("bad")
	config, ok := f.config()
	if !ok || config.Destination != (app.TunnelEndpoint{}) {
		t.Fatal("dynamic validated hidden destination")
	}
	if strings.Contains(strings.Join(f.project(newStyles(true), 72, 18).lines, "\n"), "Destination port") {
		t.Fatal("dynamic shows fixed destination")
	}
	f.update(keyPress("left"))
	if f.inputs[2].Value() != "db.internal" {
		t.Fatal("mode change lost destination")
	}
}

func TestTunnelStartupActiveCompletesOwnerWithoutStoppingManager(t *testing.T) {
	fake := &fakeTunnels{}
	m := New(Config{Tunnels: fake, NoColor: true})
	c := testConnection("captured", syntheticRootID, "/captured", 7)
	m.openTunnelForm(c, app.TunnelConfig{Mode: app.TunnelLocal, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 8080}, Destination: app.TunnelEndpoint{Host: "db.internal", Port: 5432}}, 0)
	cmd := m.startTunnelCommand(c, app.TunnelConfig{Mode: app.TunnelLocal, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 8080}, Destination: app.TunnelEndpoint{Host: "db.internal", Port: 5432}})
	batch := cmd().(tea.BatchMsg)
	m.Update(batch[1]())
	if len(m.tunnelSnapshots) != 1 || m.tunnelSnapshots[0].State != app.TunnelStarting || m.operation == nil {
		t.Fatal("Starting not published")
	}
	fake.mu.Lock()
	fake.entries[0].State = app.TunnelActive
	fake.mu.Unlock()
	m.Update(tunnelPollMsg{})
	if m.operation != nil || m.tunnelForm != nil || m.focusOwner != focusOwnerTunnels {
		t.Fatal("activation did not release input owner")
	}
	if s, _ := fake.Get(1); s.State != app.TunnelActive {
		t.Fatal("operation completion canceled active tunnel")
	}
	if fake.request.Connection.ID != c.ID || fake.request.Connection.Path != "" || fake.request.Expected == nil || *fake.request.Expected != 7 {
		t.Fatal("request did not pin ID/revision")
	}
	if fake.request.DecideTrust == nil || fake.request.ReadSecret == nil {
		t.Fatal("security owners missing")
	}
}

func TestTunnelFocusQuitStopAndIndependentSelection(t *testing.T) {
	fake := &fakeTunnels{entries: []app.TunnelSnapshot{{ID: 1, State: app.TunnelStarting, Connection: app.SSHAttemptTarget{Path: "/one"}}, {ID: 2, State: app.TunnelStopping, Connection: app.SSHAttemptTarget{Path: "/two"}}}}
	m := New(Config{Tunnels: fake, NoColor: true})
	catalog := m.browser.selectedID
	for _, want := range []focusOwner{focusOwnerDetail, focusOwnerTunnels, focusOwnerTree} {
		m.Update(keyPress("tab"))
		if m.focusOwner != want {
			t.Fatal("wrong focus cycle")
		}
	}
	m.Update(keyPress("t"))
	m.Update(keyPress("j"))
	if m.selectedTunnel != 2 || m.browser.selectedID != catalog {
		t.Fatal("selections coupled")
	}
	for _, key := range []string{"e", "m", "d", "x", "c", "f", "n", "p"} {
		_, cmd := m.Update(keyPress(key))
		if cmd != nil || m.modal.isOpen() || m.form != nil || m.tunnelForm != nil {
			t.Fatalf("catalog shortcut %s leaked", key)
		}
	}
	m.Update(keyPress("q"))
	p := m.modal.payload.(tunnelModalPayload)
	if p.action != "quit" || len(p.live) != 2 {
		t.Fatal("quit omitted Starting/Stopping")
	}
	m.Update(keyPress("enter"))
	if fake.closed {
		t.Fatal("default confirmed exit")
	}
	m.Update(keyPress("q"))
	_, cmd := m.Update(keyPress("y"))
	ready := cmd()
	_, quit := m.Update(ready)
	if _, ok := quit().(tea.QuitMsg); !ok || !fake.closed {
		t.Fatal("exit did not join manager before quit")
	}
}

func TestTunnelShellReconcilesFailureWithoutRemoteResult(t *testing.T) {
	fake := &fakeTunnels{entries: []app.TunnelSnapshot{{ID: 1, State: app.TunnelActive}}}
	m := New(Config{Tunnels: fake, NoColor: true})
	id, _, _ := m.beginOperationWith(asyncOperationSSHStart, nil, operationOwnerRoot)
	fake.mu.Lock()
	fake.entries[0].State = app.TunnelFailed
	fake.mu.Unlock()
	status := 17
	cmd := m.handleSession(sessionFinishedMsg{id: id, result: app.ConnectResult{Session: app.SSHSessionResult{RemoteExitStatus: &status}}, err: errors.New("raw transport canary")})
	if cmd != nil || m.tunnelSnapshots[0].State != app.TunnelFailed || m.sessionErr != nil || strings.Contains(m.View().Content, "raw transport canary") {
		t.Fatal("shell return did not safely reconcile")
	}
}

func TestTunnelDirtyDiscardCancelAndMinimumErrors(t *testing.T) {
	m := New(Config{Width: 40, Height: 12, NoColor: true})
	m.openTunnelForm(testConnection("host", syntheticRootID, "/host", 3), app.TunnelConfig{}, 0)
	f := m.tunnelForm
	f.inputs[1].SetValue("0")
	m.Update(keyPress("ctrl+s"))
	if f.inputs[1].Value() != "0" || f.error == "" || !strings.Contains(m.View().Content, "Listen port") {
		t.Fatal("invalid port lost value or visible field error")
	}
	m.Update(keyPress("esc"))
	if m.modal.payload.(tunnelModalPayload).action != "discard" || strings.Contains(m.View().Content, "Save") {
		t.Fatal("dirty draft offered persistence")
	}
	m.Update(keyPress("enter"))
	if m.tunnelForm != f || f.inputs[1].Value() != "0" {
		t.Fatal("Cancel lost draft")
	}
	m.Update(keyPress("esc"))
	m.Update(keyPress("d"))
	if m.tunnelForm != nil || m.focusOwner != focusOwnerTree {
		t.Fatal("Discard did not restore browser")
	}
}

func TestTunnelRetryReviewsSameCurrentIDAndMissingRecord(t *testing.T) {
	c := testConnection("pinned", syntheticRootID, "/new-path", 9)
	s := app.TunnelSnapshot{ID: 5, Attempt: 2, State: app.TunnelFailed, Connection: app.SSHAttemptTarget{ID: c.ID, Path: "/old-path", Revision: 3}, Config: app.TunnelConfig{Mode: app.TunnelDynamic, Listen: app.TunnelEndpoint{Host: "0.0.0.0", Port: 1080}, ExposureAcknowledged: true}}
	fake := &fakeTunnels{entries: []app.TunnelSnapshot{s}}
	m := New(Config{Tunnels: fake, Connections: ConnectionFuncs{GetFunc: func(_ context.Context, selector app.ItemSelector) (app.ConnectionResult, error) {
		if selector.ID != c.ID || selector.Path != "" {
			t.Fatal("retry resolved a different selector")
		}
		return app.ConnectionResult{Connection: c}, nil
	}}, NoColor: true})
	m.focusOwner = focusOwnerTunnels
	_, cmd := m.Update(keyPress("r"))
	m.Update(cmd())
	if m.tunnelForm.connection.Revision != 9 || m.tunnelForm.connection.Path != "/new-path" || m.tunnelForm.ack || m.tunnelForm.retryID != 5 {
		t.Fatal("retry did not require fresh current-record review and consent")
	}
	m.closeTunnelForm()
	m.connections = ConnectionFuncs{GetFunc: func(context.Context, app.ItemSelector) (app.ConnectionResult, error) {
		return app.ConnectionResult{}, app.ErrNotFound
	}}
	_, cmd = m.Update(keyPress("r"))
	m.Update(cmd())
	if m.tunnelForm != nil || !strings.Contains(m.View().Content, "Tunnel host is unavailable") {
		t.Fatal("missing record retry did not remain safe and visible")
	}
}

func TestTunnelStartupQuitCancelPreservesAttemptThenEscStopsOnlyIt(t *testing.T) {
	fake := &fakeTunnels{entries: []app.TunnelSnapshot{{ID: 1, Attempt: 1, State: app.TunnelActive}}}
	m := New(Config{Tunnels: fake, NoColor: true})
	c := testConnection("captured", syntheticRootID, "/captured", 7)
	config := app.TunnelConfig{Mode: app.TunnelDynamic, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 1080}}
	m.openTunnelForm(c, config, 0)
	batch := m.startTunnelCommand(c, config)().(tea.BatchMsg)
	m.Update(batch[1]())
	ctx := m.operation.ctx
	m.Update(keyPress("q"))
	m.Update(keyPress("enter"))
	if ctx.Err() != nil || m.operation == nil {
		t.Fatal("Cancel exit canceled startup")
	}
	m.Update(keyPress("esc"))
	_, poll := m.Update(tunnelPollMsg{})
	commands := poll().(tea.BatchMsg)
	m.Update(commands[0]())
	if s, _ := fake.Get(1); s.State != app.TunnelActive {
		t.Fatal("attempt cancellation stopped unrelated tunnel")
	}
	if s, _ := fake.Get(2); s.State != app.TunnelStopped {
		t.Fatal("canceled startup did not stop manager attempt")
	}
	if m.operation != nil || m.tunnelStartup != nil {
		t.Fatal("canceled startup retained owner")
	}
}

func TestTunnelStartupReusesTrustOwnerAndRejectsStaleTrust(t *testing.T) {
	fake := &fakeTunnels{}
	m := New(Config{Tunnels: fake, NoColor: true})
	c := testConnection("captured", syntheticRootID, "/captured", 7)
	config := app.TunnelConfig{Mode: app.TunnelDynamic, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 1080}}
	m.openTunnelForm(c, config, 0)
	batch := m.startTunnelCommand(c, config)().(tea.BatchMsg)
	m.Update(batch[1]())
	r := m.sessionRuntime
	response := make(chan sessionTrustResponse, 1)
	m.Update(sessionTrustRequestMsg{id: r.id, runtime: r, prompt: app.TrustDecisionPrompt{Status: app.HostTrustUnknown}, response: response})
	if m.securityInput == nil || m.securityInput.kind != securityInputTrust {
		t.Fatal("startup did not reuse trust owner")
	}
	m.Update(tea.WindowSizeMsg{Width: 39, Height: 11})
	m.Update(keyPress("o"))
	if len(response) != 0 || !m.securityInput.suspended {
		t.Fatal("undersized trust accepted input")
	}
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	_, cmd := m.Update(keyPress("enter"))
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	result := <-response
	if result.decision != app.TrustReject {
		t.Fatal("trust default was not reject")
	}
	fake.Stop(context.Background(), 1)
	m.finishTunnelStartup()
	<-done
	m.Update(sessionTrustRequestMsg{id: r.id, runtime: r, response: response})
	if m.securityInput != nil {
		t.Fatal("stale trust reclaimed input")
	}
}

func TestTunnelRunFailureClosesManager(t *testing.T) {
	fake := &fakeTunnels{entries: []app.TunnelSnapshot{{ID: 1, State: app.TunnelStarting}}}
	_, err := Run(context.Background(), Config{Tunnels: fake})
	if !errors.Is(err, app.ErrNonInteractive) || !fake.closed {
		t.Fatal("Run failure did not close manager")
	}
}

func TestTunnelMinimumStartupTrustAndQuitControlsVisible(t *testing.T) {
	fake := &fakeTunnels{}
	m := New(Config{Tunnels: fake, Width: 40, Height: 12, NoColor: true})
	c := testConnection("host", syntheticRootID, "/host", 3)
	config := app.TunnelConfig{Mode: app.TunnelDynamic, Listen: app.TunnelEndpoint{Host: "127.0.0.1", Port: 1080}}
	m.openTunnelForm(c, config, 0)
	batch := m.startTunnelCommand(c, config)().(tea.BatchMsg)
	m.Update(batch[1]())
	r := m.sessionRuntime
	m.Update(sessionTrustRequestMsg{id: r.id, runtime: r, prompt: app.TrustDecisionPrompt{Status: app.HostTrustUnknown}, response: make(chan sessionTrustResponse, 1)})
	for _, text := range []string{"y Trust once", "Enter/Esc Reject"} {
		if !strings.Contains(m.View().Content, text) {
			t.Errorf("minimum startup trust omitted %q", text)
		}
	}
	m.Update(keyPress("q"))
	for _, text := range []string{"Quit and close", "y Confirm", "Esc/Enter Cancel"} {
		if !strings.Contains(m.View().Content, text) {
			t.Errorf("startup quit omitted %q", text)
		}
	}
	m.Update(keyPress("enter"))
	if m.securityInput == nil || m.operation.ctx.Err() != nil {
		t.Fatal("Cancel lost startup security ownership")
	}
	fake.Close()
	m.finishTunnelStartup()
}

func TestTunnelStaleAdmissionStopsOnlyMatchingAttempt(t *testing.T) {
	for _, newer := range []bool{false, true} {
		fake := &fakeTunnels{entries: []app.TunnelSnapshot{{ID: 1, Attempt: 1, State: app.TunnelStarting}}}
		if newer {
			fake.entries[0].Attempt = 2
		}
		m := New(Config{Tunnels: fake, NoColor: true})
		cmd := m.handleTunnelStarted(tunnelStartedMsg{token: 900, snapshot: app.TunnelSnapshot{ID: 1, Attempt: 1, State: app.TunnelStarting}})
		if newer {
			if cmd != nil {
				t.Fatal("stale result targeted newer attempt")
			}
		} else {
			if cmd == nil {
				t.Fatal("late admitted startup leaked runtime")
			}
			m.Update(cmd())
			if s, _ := fake.Get(1); s.State != app.TunnelStopped {
				t.Fatal("late admitted startup not stopped")
			}
		}
	}
}

func TestTunnelStaleAdmissionDelayedCleanupPreservesNewerRetry(t *testing.T) {
	old := app.TunnelSnapshot{ID: 1, Attempt: 1, State: app.TunnelStarting, Connection: app.SSHAttemptTarget{ID: "host", Revision: 3}}
	fake := &fakeTunnels{entries: []app.TunnelSnapshot{old}}
	m := New(Config{Tunnels: fake, NoColor: true})
	cleanup := m.handleTunnelStarted(tunnelStartedMsg{token: 900, snapshot: old})
	if cleanup == nil {
		t.Fatal("stale admission did not schedule cleanup")
	}

	if err := fake.Stop(context.Background(), old.ID); err != nil {
		t.Fatal(err)
	}
	revision := app.Revision(3)
	newer, err := fake.Retry(context.Background(), old.ID, app.TunnelRequest{Connection: app.ItemSelector{ID: old.Connection.ID}, Expected: &revision})
	if err != nil || newer.Attempt <= old.Attempt {
		t.Fatalf("retry did not create a newer attempt: %+v, %v", newer, err)
	}
	fake.mu.Lock()
	fake.entries[0].State = app.TunnelActive
	newer = fake.entries[0]
	fake.mu.Unlock()

	m.Update(cleanup())
	if len(fake.stopAttempts) != 1 || fake.stopAttempts[0] != [2]uint64{old.ID, old.Attempt} {
		t.Fatalf("cleanup did not atomically target captured attempt: %v", fake.stopAttempts)
	}
	if got, ok := fake.Get(old.ID); !ok || got != newer {
		t.Fatalf("delayed stale cleanup changed newer active retry: %+v, want %+v", got, newer)
	}
	if len(fake.stops) != 1 {
		t.Fatal("delayed stale cleanup stopped the retried tunnel")
	}
}

func TestTunnelConfirmedStopDismissAndInspectionOverflow(t *testing.T) {
	s := app.TunnelSnapshot{ID: 1, State: app.TunnelActive, Connection: app.SSHAttemptTarget{ID: "host", Path: "/host", Host: "ssh.test", Port: 22}, Config: app.TunnelConfig{Mode: app.TunnelRemote, Listen: app.TunnelEndpoint{Host: "::1", Port: 8080}, Destination: app.TunnelEndpoint{Host: "destination-with-complete-value.test", Port: 5432}}, Warning: "scope warning"}
	fake := &fakeTunnels{entries: []app.TunnelSnapshot{s, {ID: 2, State: app.TunnelActive}}}
	m := New(Config{Tunnels: fake, Width: 40, Height: 12, NoColor: true})
	m.focusOwner = focusOwnerTunnels
	catalog := m.browser.selectedID
	m.Update(keyPress("enter"))
	body := m.modal.payload.(tunnelModalPayload).lines(m.styles, m.layout().modalOverlay().contentWidth())
	complete := strings.Join(strings.Fields(strings.Join(body, "")), "")
	if !strings.Contains(complete, s.Config.Destination.String()) {
		t.Fatal("inspection discarded destination bytes")
	}
	var inspected strings.Builder
	for range 80 {
		inspected.WriteString(m.View().Content)
		m.Update(keyPress("j"))
	}
	for _, text := range []string{"Requested listener", "unverified", "[::1]:8080"} {
		if !strings.Contains(inspected.String(), text) {
			t.Errorf("inspection cannot reach %q", text)
		}
	}
	for _, line := range body {
		if strings.HasPrefix(line, modalControlsPrefix) {
			continue
		}
		if text := strings.TrimSpace(line); text != "" && !strings.Contains(inspected.String(), text) {
			t.Errorf("inspection cannot reach complete wrapped line %q", text)
		}
	}
	m.Update(keyPress("s"))
	m.Update(keyPress("enter"))
	if len(fake.stops) != 0 {
		t.Fatal("default stop choice was not Cancel")
	}
	m.Update(keyPress("s"))
	_, cmd := m.Update(keyPress("y"))
	m.Update(cmd())
	if first, _ := fake.Get(1); first.State != app.TunnelStopped {
		t.Fatal("confirmed stop did not stop selected ID")
	}
	if other, _ := fake.Get(2); other.State != app.TunnelActive {
		t.Fatal("stop affected unrelated tunnel")
	}
	_, cmd = m.Update(keyPress("d"))
	m.Update(cmd())
	if _, ok := fake.Get(1); ok || m.selectedTunnel != 2 || m.browser.selectedID != catalog {
		t.Fatal("dismiss did not move stable selection without catalog mutation")
	}
}

type joiningTunnels struct {
	fakeTunnels
	entered, release chan struct{}
	once             sync.Once
}

func (f *joiningTunnels) Close() error {
	f.once.Do(func() { close(f.entered); <-f.release; _ = f.fakeTunnels.Close() })
	return nil
}

func TestTunnelRunRootCancellationWaitsForManagerJoin(t *testing.T) {
	service := &joiningTunnels{entered: make(chan struct{}), release: make(chan struct{}), fakeTunnels: fakeTunnels{entries: []app.TunnelSnapshot{{ID: 1, State: app.TunnelStarting}}}}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := Run(ctx, Config{Tunnels: service, Terminal: terminal.NewFake(terminal.Size{Columns: 80, Rows: 24}), Stdout: io.Discard})
		done <- err
	}()
	cancel()
	select {
	case <-service.entered:
	case <-time.After(3 * time.Second):
		close(service.release)
		t.Fatal("root cancellation did not reach manager")
	}
	select {
	case <-done:
		close(service.release)
		t.Fatal("Run returned before manager joined")
	default:
	}
	close(service.release)
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("root cancellation returned successful exit")
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run did not return after join")
	}
	if !service.closed {
		t.Fatal("root cancellation left Starting runtime")
	}
}

func TestTunnelUndersizedQuitCannotConfirmHiddenTargets(t *testing.T) {
	fake := &fakeTunnels{entries: []app.TunnelSnapshot{{ID: 1, State: app.TunnelStarting}}}
	m := New(Config{Tunnels: fake, Width: 39, Height: 11, NoColor: true})
	m.Update(keyPress("q"))
	_, cmd := m.Update(keyPress("y"))
	if cmd != nil || fake.closed {
		t.Fatal("undersized quit confirmed targets that were not visible")
	}
	if !strings.Contains(m.View().Content, "resize to review") {
		t.Fatal("undersized live exit omitted review guidance")
	}
	m.Update(keyPress("enter"))
	if m.modal.isOpen() || fake.closed {
		t.Fatal("undersized default choice did not cancel")
	}
}
