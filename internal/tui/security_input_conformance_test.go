package tui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
)

const securityInputCanary = "T085-terminal-owned-secret-canary"

type securityConformanceTerminal struct {
	mu        sync.Mutex
	size      terminal.Size
	resizes   []terminal.Size
	secret    []byte
	err       error
	resizeErr error
	inspect   func(terminal.SecretPrompt)
	reads     int
}

func (t *securityConformanceTerminal) Interactive() bool { return true }
func (t *securityConformanceTerminal) Capture(context.Context) (terminal.State, error) {
	return terminal.State{}, nil
}
func (t *securityConformanceTerminal) VTCapability(context.Context) (terminal.VTCapability, error) {
	return terminal.VTCapability{Status: terminal.VTSupported}, nil
}
func (t *securityConformanceTerminal) Size(ctx context.Context) (terminal.Size, error) {
	if err := ctx.Err(); err != nil {
		return terminal.Size{}, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.size, nil
}
func (t *securityConformanceTerminal) MakeRaw(context.Context) error { return nil }
func (t *securityConformanceTerminal) Restore(context.Context, terminal.State) error {
	return nil
}
func (t *securityConformanceTerminal) ReadSecret(ctx context.Context, prompt terminal.SecretPrompt) ([]byte, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	t.mu.Lock()
	t.reads++
	inspect, err := t.inspect, t.err
	secret := append([]byte(nil), t.secret...)
	t.mu.Unlock()
	if inspect != nil {
		inspect(prompt)
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		wipeSecret(secret)
		return nil, ctxErr
	}
	if err != nil {
		wipeSecret(secret)
		return nil, err
	}
	return secret, nil
}
func (t *securityConformanceTerminal) ResizeEvents(ctx context.Context) (<-chan terminal.Size, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.resizeErr != nil {
		return nil, t.resizeErr
	}
	events := make(chan terminal.Size, len(t.resizes))
	for _, size := range t.resizes {
		events <- size
		t.size = size
	}
	close(events)
	return events, nil
}

type securityDecisionReader struct {
	value   string
	ctx     context.Context
	inspect func()
}

func (r securityDecisionReader) Read(value []byte) (int, error) {
	if r.inspect != nil {
		r.inspect()
	}
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return strings.NewReader(r.value).Read(value)
}

func TestT085TrustSecurityInputIntegratedMatrixTwentyRuns(t *testing.T) {
	statuses := []app.HostTrustStatus{app.HostTrustUnknown, app.HostTrustChanged}
	cases := []struct {
		name     string
		input    string
		want     app.TrustDecision
		size     terminal.Size
		resizes  []terminal.Size
		terminal error
	}{
		{name: "accept", input: "once\n", want: app.TrustOnce, size: terminal.Size{Columns: 100, Rows: 30}},
		{name: "cancel", input: "\n", want: app.TrustReject, size: terminal.Size{Columns: 100, Rows: 30}},
		{name: "normal", input: "once\n", want: app.TrustOnce, size: terminal.Size{Columns: 100, Rows: 30}},
		{name: "reduced", input: "once\n", want: app.TrustOnce, size: terminal.Size{Columns: 60, Rows: 20}},
		{name: "normal-to-reduced", input: "once\n", want: app.TrustOnce, size: terminal.Size{Columns: 100, Rows: 30}, resizes: []terminal.Size{{Columns: 60, Rows: 20}}},
		{name: "reduced-to-normal", input: "once\n", want: app.TrustOnce, size: terminal.Size{Columns: 60, Rows: 20}, resizes: []terminal.Size{{Columns: 100, Rows: 30}}},
		{name: "undersized-recovery", input: "once\n", want: app.TrustOnce, size: terminal.Size{Columns: 39, Rows: 11}, resizes: []terminal.Size{{Columns: 60, Rows: 20}}},
		{name: "terminal-failure", want: app.TrustReject, size: terminal.Size{Columns: 39, Rows: 11}, terminal: errors.New("T085 trust terminal failure")},
	}

	for _, status := range statuses {
		for _, test := range cases {
			t.Run(string(status)+"/"+test.name, func(t *testing.T) {
				for run := 0; run < 20; run++ {
					model, command, output := newSecurityConformanceCommand(status, app.SecretKind(""), test.size, test.resizes, test.terminal)
					command.input = securityDecisionReader{value: test.input, ctx: command.ctx, inspect: func() {
						assertSecurityCommandOwnership(t, model, command, securityInputTrust)
						assertStaleSecurityResultIsInert(t, model)
					}}
					err := command.Run()
					if test.terminal != nil {
						if !errors.Is(err, test.terminal) {
							t.Fatalf("run %d error = %v, want terminal failure", run, err)
						}
					} else if err != nil {
						t.Fatalf("run %d command failed: %v", run, err)
					}
					if test.terminal == nil && command.result.Connection.Name != string(test.want) {
						t.Fatalf("run %d decision = %q, want %q", run, command.result.Connection.Name, test.want)
					}
					assertSecurityCommandClean(t, model, command, output.String())
				}
			})
		}
	}
}

func TestT085SecretSecurityInputIntegratedMatrixTwentyRuns(t *testing.T) {
	kinds := []app.SecretKind{app.SecretPassword, app.SecretPassphrase}
	cases := []struct {
		name     string
		size     terminal.Size
		resizes  []terminal.Size
		terminal error
	}{
		{name: "masked-terminal-owned", size: terminal.Size{Columns: 100, Rows: 30}},
		{name: "cancel", size: terminal.Size{Columns: 100, Rows: 30}, terminal: context.Canceled},
		{name: "normal", size: terminal.Size{Columns: 100, Rows: 30}},
		{name: "reduced", size: terminal.Size{Columns: 60, Rows: 20}},
		{name: "normal-to-reduced", size: terminal.Size{Columns: 100, Rows: 30}, resizes: []terminal.Size{{Columns: 60, Rows: 20}}},
		{name: "reduced-to-normal", size: terminal.Size{Columns: 60, Rows: 20}, resizes: []terminal.Size{{Columns: 100, Rows: 30}}},
		{name: "undersized-recovery", size: terminal.Size{Columns: 39, Rows: 11}, resizes: []terminal.Size{{Columns: 60, Rows: 20}}},
		{name: "terminal-failure", size: terminal.Size{Columns: 100, Rows: 30}, terminal: errors.New("T085 secret terminal failure")},
	}

	for _, kind := range kinds {
		for _, test := range cases {
			t.Run(string(kind)+"/"+test.name, func(t *testing.T) {
				for run := 0; run < 20; run++ {
					model, command, output := newSecurityConformanceCommand("", kind, test.size, test.resizes, test.terminal)
					local := command.terminal.(*securityConformanceTerminal)
					local.inspect = func(prompt terminal.SecretPrompt) {
						assertSecurityCommandOwnership(t, model, command, securityInputSecret)
						state := command.securityInputSnapshot()
						if state.secret == nil || state.secret.kind != kind || state.secret.masked != 0 {
							t.Fatalf("run %d secret presentation retained terminal input: %#v", run, state.secret)
						}
						if kind == app.SecretPassword && prompt.Message != "Password" || kind == app.SecretPassphrase && prompt.Message != "Private key passphrase" {
							t.Fatalf("run %d prompt = %q", run, prompt.Message)
						}
						assertStaleSecurityResultIsInert(t, model)
					}
					err := command.Run()
					if test.terminal != nil {
						if !errors.Is(err, test.terminal) {
							t.Fatalf("run %d error = %v, want %v", run, err, test.terminal)
						}
					} else if err != nil {
						t.Fatalf("run %d command failed: %v", run, err)
					}
					assertSecurityCommandClean(t, model, command, output.String())
				}
			})
		}
	}
}

func TestT085SecurityInputOperationCancellationTwentyRuns(t *testing.T) {
	variants := []struct {
		name       string
		owner      securityInputKind
		status     app.HostTrustStatus
		secretKind app.SecretKind
	}{
		{name: "trust-unknown", owner: securityInputTrust, status: app.HostTrustUnknown},
		{name: "trust-changed", owner: securityInputTrust, status: app.HostTrustChanged},
		{name: "password", owner: securityInputSecret, secretKind: app.SecretPassword},
		{name: "passphrase", owner: securityInputSecret, secretKind: app.SecretPassphrase},
	}
	for _, variant := range variants {
		t.Run(variant.name, func(t *testing.T) {
			for run := 0; run < 20; run++ {
				model, command, output := newSecurityConformanceCommand(variant.status, variant.secretKind, terminal.Size{Columns: 100, Rows: 30}, nil, nil)
				if variant.owner == securityInputTrust {
					command.input = securityDecisionReader{value: "once\n", ctx: command.ctx, inspect: func() {
						assertSecurityCommandOwnership(t, model, command, variant.owner)
						model.cancelCurrentOperation(false)
					}}
				} else {
					command.terminal.(*securityConformanceTerminal).inspect = func(terminal.SecretPrompt) {
						assertSecurityCommandOwnership(t, model, command, variant.owner)
						model.cancelCurrentOperation(false)
					}
				}
				if err := command.Run(); !errors.Is(err, context.Canceled) {
					t.Fatalf("run %d cancellation error = %v", run, err)
				}
				if model.operation == nil || model.operation.phase != asyncPhaseCancelRequested || model.focusOwner != focusOwnerDetail {
					t.Fatalf("run %d cancellation changed ownership: operation=%+v focus=%v", run, model.operation, model.focusOwner)
				}
				assertSecurityCommandClean(t, model, command, output.String())
			}
		})
	}
}

func TestT085TrustTerminalReadFailurePropagates(t *testing.T) {
	want := errors.New("T085 trust read failure")
	decision, err := decideTrust(context.Background(), errorReader{err: want}, io.Discard, app.TrustDecisionPrompt{Status: app.HostTrustUnknown}, true)
	if decision != app.TrustReject || !errors.Is(err, want) {
		t.Fatalf("decision/error = %q/%v, want reject/read failure", decision, err)
	}
}

type errorReader struct{ err error }

func (r errorReader) Read([]byte) (int, error) { return 0, r.err }

func newSecurityConformanceCommand(status app.HostTrustStatus, secretKind app.SecretKind, size terminal.Size, resizes []terminal.Size, terminalErr error) (*Model, *sessionExecCommand, *bytes.Buffer) {
	local := &securityConformanceTerminal{
		size: size, resizes: append([]terminal.Size(nil), resizes...), secret: []byte(securityInputCanary), err: terminalErr,
	}
	if status != "" {
		local.resizeErr = terminalErr
		local.err = nil
	}
	model := New(Config{Terminal: local, Width: 100, Height: 30, NoColor: true})
	model.focusOwner = focusOwnerDetail
	target := capturedTarget{id: "server", revision: 7, kind: app.NodeKindConnection, path: "/prod/server", endpointOrScope: "server.test:22"}
	_, operationCtx, _ := model.beginOperationWith(asyncOperationSSHStart, &target, operationOwnerModal)
	output := new(bytes.Buffer)
	command := &sessionExecCommand{
		ctx: operationCtx, terminal: local, output: output, errOut: io.Discard, noColor: true,
		preservedFocus: model.focusOwner, width: model.width, height: model.height,
	}
	command.service = ConnectFunc(func(ctx context.Context, request app.ConnectRequest) (app.ConnectResult, error) {
		if status != "" {
			decision, err := request.DecideTrust(ctx, app.TrustDecisionPrompt{Status: status, Host: app.PresentedHost{Endpoint: app.HostEndpoint{CanonicalHost: "server.test", Port: 22}, FingerprintSHA256: "SHA256:public"}})
			if err != nil {
				return app.ConnectResult{}, err
			}
			return app.ConnectResult{Connection: app.Connection{Node: app.Node{Name: string(decision)}}}, nil
		}
		secret, err := request.ReadSecret(ctx, app.SecretRequest{Kind: secretKind})
		if err != nil {
			wipeSecret(secret)
			return app.ConnectResult{}, err
		}
		matched := string(secret) == securityInputCanary
		wipeSecret(secret)
		if !matched {
			return app.ConnectResult{}, errors.New("terminal secret changed before use")
		}
		return app.ConnectResult{}, nil
	})
	return model, command, output
}

func assertSecurityCommandOwnership(t *testing.T, model *Model, command *sessionExecCommand, kind securityInputKind) {
	t.Helper()
	state := command.securityInputSnapshot()
	if !state.preemptsApplicationInput() || state.kind != kind || state.preservedFocus != focusOwnerDetail {
		t.Fatalf("security owner = %#v, want active %v preserving Details", state, kind)
	}
	if model.operation == nil || model.operation.kind != asyncOperationSSHStart || model.operation.owner != operationOwnerModal || model.focusOwner != focusOwnerDetail {
		t.Fatalf("application ownership changed: operation=%+v focus=%v", model.operation, model.focusOwner)
	}
	if state.suspended || state.width < minimumLayoutWidth || state.height < minimumLayoutHeight {
		t.Fatalf("security input resumed at invalid size: %#v", state)
	}
}

func assertStaleSecurityResultIsInert(t *testing.T, model *Model) {
	t.Helper()
	id := model.operation.id
	updateModel(model, sessionFinishedMsg{id: id + 1, err: errors.New("stale security result")})
	if model.operation == nil || model.operation.id != id || model.focusOwner != focusOwnerDetail || model.modal.isOpen() {
		t.Fatal("stale result changed security/application ownership")
	}
}

func assertSecurityCommandClean(t *testing.T, model *Model, command *sessionExecCommand, output string) {
	t.Helper()
	state := command.securityInputSnapshot()
	if state.preemptsApplicationInput() || state.trust != nil || state.secret != nil {
		t.Fatalf("security input was not cleaned up: %#v", state)
	}
	if model.focusOwner != focusOwnerDetail || model.operation == nil || model.operation.kind != asyncOperationSSHStart {
		t.Fatalf("command changed root ownership: focus=%v operation=%+v", model.focusOwner, model.operation)
	}
	for _, exposed := range []string{
		model.View().Content, output, errorText(command.err),
		failureText(command.result.Session.Failure),
	} {
		if strings.Contains(exposed, securityInputCanary) {
			t.Fatalf("secret escaped terminal ownership: %q", exposed)
		}
	}
}

func failureText(failure *app.SSHFailurePresentation) string {
	if failure == nil {
		return ""
	}
	return failure.Summary + failure.Recommendation + failure.TechnicalDetail
}

func errorText(value any) string {
	if err, ok := value.(error); ok {
		return err.Error()
	}
	return ""
}
