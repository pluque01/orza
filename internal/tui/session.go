package tui

import (
	"context"
	"errors"
	"io"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
)

type sessionFinishedMsg struct {
	id              uint64
	attempt         app.SSHAttemptTarget
	recoveryAttempt app.SSHAttemptTarget
	recoveryFailure *app.SSHFailurePresentation
	result          app.ConnectResult
	err             error
}

type sessionExecCommand struct {
	ctx                context.Context
	service            ConnectService
	terminal           app.Terminal
	request            app.ConnectRequest
	input              io.Reader
	output             io.Writer
	errOut             io.Writer
	noColor            bool
	preservedFocus     focusOwner
	width              int
	height             int
	result             app.ConnectResult
	err                error
	securityMu         sync.Mutex
	securityInput      securityInputState
	securityCancel     context.CancelFunc
	securityReadCancel context.CancelFunc
	securityDone       chan struct{}
	securityChanged    chan struct{}
	securityEpoch      uint64
}

func (c *sessionExecCommand) SetStdin(input io.Reader)   { c.input = input }
func (c *sessionExecCommand) SetStdout(output io.Writer) { c.output = output }
func (c *sessionExecCommand) SetStderr(output io.Writer) { c.errOut = output }

func (c *sessionExecCommand) Run() error {
	if c.service == nil {
		c.err = errUnavailable
		return c.err
	}
	decide := func(ctx context.Context, prompt app.TrustDecisionPrompt) (app.TrustDecision, error) {
		if err := c.beginSecurityInput(ctx, securityInputTrust); err != nil {
			return app.TrustReject, err
		}
		c.securityMu.Lock()
		c.securityInput.trust = newTrustPrompt(prompt)
		c.securityMu.Unlock()
		defer c.endSecurityInput()
		for {
			c.securityMu.Lock()
			epoch := c.securityEpoch
			c.securityMu.Unlock()
			decision, err := decideTrust(ctx, c.input, c.output, prompt, c.noColor)
			c.securityMu.Lock()
			stale := c.securityInput.suspended || c.securityEpoch != epoch
			c.securityMu.Unlock()
			if !stale || err != nil {
				return decision, err
			}
			if err := c.waitSecurityInput(ctx); err != nil {
				return app.TrustReject, err
			}
		}
	}
	c.request.DecideTrust = decide
	c.request.ReadSecret = c.readSecret
	c.result, c.err = c.service.Connect(c.ctx, c.request)
	return c.err
}

func (c *sessionExecCommand) readSecret(ctx context.Context, request app.SecretRequest) ([]byte, error) {
	if c.terminal == nil {
		return nil, errUnavailable
	}
	if err := c.beginSecurityInput(ctx, securityInputSecret); err != nil {
		return nil, err
	}
	c.securityMu.Lock()
	c.securityInput.secret = newSecretPrompt(request.Kind, "")
	c.securityMu.Unlock()
	defer c.endSecurityInput()
	prompt := request.Prompt
	if prompt == "" {
		if request.Kind == app.SecretPassphrase {
			prompt = "Private key passphrase"
		} else {
			prompt = "Password"
		}
	}
	readCtx, cancel := context.WithCancel(ctx)
	c.securityMu.Lock()
	c.securityReadCancel = cancel
	if c.securityInput.suspended {
		cancel()
	}
	c.securityMu.Unlock()
	secret, err := c.terminal.ReadSecret(readCtx, terminal.SecretPrompt{Message: prompt})
	c.securityMu.Lock()
	suspended := c.securityInput.suspended
	c.securityReadCancel = nil
	c.securityMu.Unlock()
	cancel()
	if suspended && ctx.Err() == nil {
		wipeSecret(secret)
		return nil, context.Canceled
	}
	return secret, err
}

func (c *sessionExecCommand) beginSecurityInput(ctx context.Context, kind securityInputKind) error {
	state, ok := newSecurityInputState(kind, c.preservedFocus)
	if !ok {
		return errUnavailable
	}
	width, height := c.width, c.height
	if c.terminal != nil {
		size, err := c.terminal.Size(ctx)
		if err != nil {
			return err
		}
		width, height = size.Columns, size.Rows
	}
	c.securityMu.Lock()
	c.securityInput = state.resize(width, height)
	c.securityChanged = make(chan struct{}, 1)
	suspended := c.securityInput.suspended
	c.securityMu.Unlock()
	if c.terminal == nil {
		if suspended {
			c.endSecurityInput()
			return errUnavailable
		}
		return nil
	}
	resizeCtx, cancel := context.WithCancel(ctx)
	resizes, err := c.terminal.ResizeEvents(resizeCtx)
	if err != nil {
		cancel()
		c.endSecurityInput()
		return err
	}
	c.securityCancel = cancel
	if suspended {
		message := strings.Join([]string{"Terminal too small", "Required minimum: 40x12", "Resize to at least 40x12.", ""}, "\n")
		if _, err := io.WriteString(c.output, message); err != nil {
			c.endSecurityInput()
			return err
		}
	}
	closed := false
	for suspended {
		select {
		case <-ctx.Done():
			c.endSecurityInput()
			return ctx.Err()
		case size, open := <-resizes:
			if !open {
				c.endSecurityInput()
				return io.EOF
			}
			if size.Valid() {
				c.resizeSecurityInput(size)
				c.securityMu.Lock()
				suspended = c.securityInput.suspended
				c.securityMu.Unlock()
			}
		}
	}
	c.securityDone = make(chan struct{})
	for !closed {
		select {
		case size, open := <-resizes:
			if !open {
				closed = true
				continue
			}
			if size.Valid() {
				c.resizeSecurityInput(size)
			}
		default:
			go c.watchSecurityInputResizes(resizes)
			return nil
		}
	}
	close(c.securityDone)
	return nil
}

func (c *sessionExecCommand) endSecurityInput() {
	if c.securityCancel != nil {
		c.securityCancel()
	}
	if c.securityDone != nil {
		<-c.securityDone
	}
	c.securityMu.Lock()
	defer c.securityMu.Unlock()
	if c.securityInput.secret != nil {
		c.securityInput.secret.clear()
	}
	c.securityInput = securityInputState{}
	c.securityCancel = nil
	c.securityReadCancel = nil
	c.securityDone = nil
	c.securityChanged = nil
}

func (c *sessionExecCommand) watchSecurityInputResizes(resizes <-chan terminal.Size) {
	defer close(c.securityDone)
	for size := range resizes {
		if size.Valid() {
			c.resizeSecurityInput(size)
		}
	}
}

func (c *sessionExecCommand) resizeSecurityInput(size terminal.Size) {
	c.securityMu.Lock()
	wasSuspended := c.securityInput.suspended
	c.securityInput = c.securityInput.resize(size.Columns, size.Rows)
	if c.securityInput.suspended && !wasSuspended {
		c.securityEpoch++
		if c.securityReadCancel != nil {
			c.securityReadCancel()
		}
	}
	if c.securityChanged != nil {
		select {
		case c.securityChanged <- struct{}{}:
		default:
		}
	}
	c.securityMu.Unlock()
}

func (c *sessionExecCommand) waitSecurityInput(ctx context.Context) error {
	for {
		c.securityMu.Lock()
		suspended := c.securityInput.suspended
		changed := c.securityChanged
		c.securityMu.Unlock()
		if !suspended {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-changed:
		}
	}
}

func (c *sessionExecCommand) securityInputSnapshot() securityInputState {
	c.securityMu.Lock()
	defer c.securityMu.Unlock()
	return c.securityInput
}

type sessionTrustResponse struct {
	decision app.TrustDecision
	err      error
}

type sessionSecretResponse struct {
	secret []byte
	err    error
}

type sessionTrustRequestMsg struct {
	id       uint64
	runtime  *sessionRuntime
	prompt   app.TrustDecisionPrompt
	response chan sessionTrustResponse
}

type sessionSecretRequestMsg struct {
	id       uint64
	runtime  *sessionRuntime
	request  app.SecretRequest
	response chan sessionSecretResponse
	discard  bool
}

type sessionActivateMsg struct {
	id      uint64
	runtime *sessionRuntime
	ready   chan struct{}
}

type sessionRuntimeClosedMsg struct {
	id      uint64
	runtime *sessionRuntime
}

type sessionSecretFinishedMsg struct {
	id        uint64
	runtime   *sessionRuntime
	suspended bool
	size      terminal.Size
	retry     bool
	err       error
}

type sessionJoinedMsg struct {
	id      uint64
	runtime *sessionRuntime
	err     error
}

type sessionRuntime struct {
	id              uint64
	ctx             context.Context
	service         ConnectService
	request         app.ConnectRequest
	requests        chan tea.Msg
	done            chan struct{}
	result          app.ConnectResult
	err             error
	attempt         app.SSHAttemptTarget
	recoveryAttempt app.SSHAttemptTarget
	recoveryFailure *app.SSHFailurePresentation
}

func (r *sessionRuntime) run() tea.Msg {
	defer close(r.done)
	if r.service == nil {
		r.err = errUnavailable
	} else {
		r.request.DecideTrust = r.decideTrust
		r.request.ReadSecret = r.readSecret
		r.request.Activate = r.activate
		r.result, r.err = r.service.Connect(r.ctx, r.request)
	}
	return sessionFinishedMsg{
		id: r.id, attempt: r.attempt, recoveryAttempt: r.recoveryAttempt,
		recoveryFailure: r.recoveryFailure, result: r.result, err: r.err,
	}
}

func (r *sessionRuntime) decideTrust(ctx context.Context, prompt app.TrustDecisionPrompt) (app.TrustDecision, error) {
	response := make(chan sessionTrustResponse, 1)
	if !r.send(ctx, sessionTrustRequestMsg{id: r.id, runtime: r, prompt: prompt, response: response}) {
		return app.TrustReject, ctx.Err()
	}
	select {
	case <-ctx.Done():
		return app.TrustReject, ctx.Err()
	case result := <-response:
		return result.decision, result.err
	}
}

func (r *sessionRuntime) readSecret(ctx context.Context, request app.SecretRequest) ([]byte, error) {
	response := make(chan sessionSecretResponse, 1)
	if !r.send(ctx, sessionSecretRequestMsg{id: r.id, runtime: r, request: request, response: response}) {
		return nil, ctx.Err()
	}
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-response:
		return result.secret, result.err
	}
}

func (r *sessionRuntime) activate(ctx context.Context) error {
	ready := make(chan struct{})
	if !r.send(ctx, sessionActivateMsg{id: r.id, runtime: r, ready: ready}) {
		return ctx.Err()
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-ready:
		return nil
	}
}

func (r *sessionRuntime) send(ctx context.Context, msg tea.Msg) bool {
	select {
	case <-ctx.Done():
		return false
	case r.requests <- msg:
		return true
	}
}

func (r *sessionRuntime) wait() tea.Msg {
	select {
	case msg := <-r.requests:
		return msg
	case <-r.done:
		return sessionRuntimeClosedMsg{id: r.id, runtime: r}
	}
}

type sessionJoinCommand struct {
	runtime *sessionRuntime
	ready   chan struct{}
	input   io.Reader
	output  io.Writer
	errOut  io.Writer
}

func (c *sessionJoinCommand) SetStdin(input io.Reader)   { c.input = input }
func (c *sessionJoinCommand) SetStdout(output io.Writer) { c.output = output }
func (c *sessionJoinCommand) SetStderr(output io.Writer) { c.errOut = output }
func (c *sessionJoinCommand) Run() error {
	close(c.ready)
	<-c.runtime.done
	return nil
}

type gatedSecretCommand struct {
	ctx       context.Context
	terminal  app.Terminal
	prompt    terminal.SecretPrompt
	discard   bool
	consume   func([]byte, error)
	input     io.Reader
	output    io.Writer
	errOut    io.Writer
	suspended bool
	retry     bool
	size      terminal.Size
	err       error
}

func (c *gatedSecretCommand) SetStdin(input io.Reader)   { c.input = input }
func (c *gatedSecretCommand) SetStdout(output io.Writer) { c.output = output }
func (c *gatedSecretCommand) SetStderr(output io.Writer) { c.errOut = output }

func (c *gatedSecretCommand) Run() error {
	if c.terminal == nil {
		c.err = errUnavailable
		c.consume(nil, c.err)
		return nil
	}
	size, err := c.terminal.Size(c.ctx)
	if err != nil {
		c.err = err
		c.consume(nil, err)
		return nil
	}
	c.size = size
	if undersizedTerminal(size) {
		c.suspended = true
		return nil
	}
	resizeCtx, stopResize := context.WithCancel(c.ctx)
	resizes, err := c.terminal.ResizeEvents(resizeCtx)
	if err != nil {
		stopResize()
		c.err = err
		c.consume(nil, err)
		return nil
	}
	readCtx, stopRead := context.WithCancel(c.ctx)
	type secretResult struct {
		secret []byte
		err    error
	}
	result := make(chan secretResult, 1)
	go func() {
		secret, readErr := c.terminal.ReadSecret(readCtx, c.prompt)
		result <- secretResult{secret: secret, err: readErr}
	}()
	for {
		select {
		case <-c.ctx.Done():
			stopRead()
			read := <-result
			wipeSecret(read.secret)
			stopAndJoinResizeInput(stopResize, resizes)
			c.err = c.ctx.Err()
			c.consume(nil, c.err)
			return nil
		case size, open := <-resizes:
			if !open {
				resizes = nil
				continue
			}
			if !size.Valid() {
				continue
			}
			c.size = size
			if undersizedTerminal(size) {
				stopRead()
				read := <-result
				wipeSecret(read.secret)
				stopAndJoinResizeInput(stopResize, resizes)
				c.suspended = true
				return nil
			}
		case read := <-result:
			stopRead()
			stopAndJoinResizeInput(stopResize, resizes)
			current, sizeErr := c.terminal.Size(c.ctx)
			if sizeErr == nil {
				c.size = current
			}
			if sizeErr == nil && undersizedTerminal(current) {
				wipeSecret(read.secret)
				c.suspended = true
				return nil
			}
			if read.err != nil {
				wipeSecret(read.secret)
				c.err = read.err
				c.consume(nil, read.err)
				return nil
			}
			if c.discard {
				wipeSecret(read.secret)
				c.retry = true
				return nil
			}
			c.consume(read.secret, nil)
			return nil
		}
	}
}

func undersizedTerminal(size terminal.Size) bool {
	return size.Columns < minimumLayoutWidth || size.Rows < minimumLayoutHeight
}

func stopAndJoinResizeInput(stop context.CancelFunc, resizes <-chan terminal.Size) {
	stop()
	if resizes == nil {
		return
	}
	for range resizes {
	}
}

func (m *Model) acceptsSessionRuntime(id uint64, runtime *sessionRuntime) bool {
	return runtime != nil && runtime == m.sessionRuntime && m.operation != nil &&
		m.operation.matchesResult(id, asyncOperationSSHStart)
}

func (m *Model) handleSessionTrustRequest(msg sessionTrustRequestMsg) tea.Cmd {
	if !m.acceptsSessionRuntime(msg.id, msg.runtime) {
		return nil
	}
	if err := msg.runtime.ctx.Err(); err != nil {
		return m.respondTrust(msg, app.TrustReject, err)
	}
	state, ok := newSecurityInputState(securityInputTrust, m.focusOwner)
	if !ok {
		return m.respondTrust(msg, app.TrustReject, errUnavailable)
	}
	state = state.resize(m.width, m.height)
	state.trust = newTrustPrompt(msg.prompt)
	m.securityInput = &state
	m.sessionTrustResponse = msg.response
	return nil
}

func (m *Model) handleSessionSecretRequest(msg sessionSecretRequestMsg) tea.Cmd {
	if !m.acceptsSessionRuntime(msg.id, msg.runtime) {
		return nil
	}
	if err := msg.runtime.ctx.Err(); err != nil {
		return m.respondSecret(msg, nil, err)
	}
	state, ok := newSecurityInputState(securityInputSecret, m.focusOwner)
	if !ok {
		return m.respondSecret(msg, nil, errUnavailable)
	}
	state = state.resize(m.width, m.height)
	state.secret = newSecretPrompt(msg.request.Kind, "")
	m.securityInput = &state
	pending := msg
	m.sessionSecret = &pending
	if state.suspended {
		return nil
	}
	return m.startSessionSecretInput()
}

func (m *Model) startSessionSecretInput() tea.Cmd {
	if m.securityInput == nil || m.securityInput.suspended || m.sessionSecret == nil || m.operation == nil || m.operation.ctx == nil {
		return nil
	}
	pending := *m.sessionSecret
	prompt := pending.request.Prompt
	if prompt == "" {
		if pending.request.Kind == app.SecretPassphrase {
			prompt = "Private key passphrase"
		} else {
			prompt = "Password"
		}
	}
	command := &gatedSecretCommand{
		ctx: m.operation.ctx, terminal: m.terminal, prompt: terminal.SecretPrompt{Message: prompt}, discard: pending.discard,
		consume: func(secret []byte, err error) {
			if err != nil {
				wipeSecret(secret)
				return
			}
			select {
			case pending.response <- sessionSecretResponse{secret: secret}:
			case <-pending.runtime.ctx.Done():
				wipeSecret(secret)
			}
		},
	}
	return tea.Exec(command, func(execErr error) tea.Msg {
		if command.err == nil && execErr != nil {
			command.err = execErr
		}
		return sessionSecretFinishedMsg{
			id: pending.id, runtime: pending.runtime, suspended: command.suspended,
			size: command.size, retry: command.retry, err: command.err,
		}
	})
}

func (m *Model) handleSessionSecretFinished(msg sessionSecretFinishedMsg) tea.Cmd {
	if !m.acceptsSessionRuntime(msg.id, msg.runtime) || m.sessionSecret == nil {
		return nil
	}
	if msg.suspended {
		m.width, m.height = msg.size.Columns, msg.size.Rows
		state := m.securityInput.resize(m.width, m.height)
		m.securityInput = &state
		m.sessionSecret.discard = true
		return nil
	}
	if msg.retry {
		m.sessionSecret.discard = false
		return m.startSessionSecretInput()
	}
	pending := *m.sessionSecret
	m.clearSessionSecurityInput()
	if errors.Is(msg.err, terminal.ErrSecretQuit) {
		m.cancelCurrentOperation(true)
		return m.respondSecret(pending, nil, context.Canceled)
	} else if msg.err != nil {
		m.cancelCurrentOperation(false)
		return m.respondSecret(pending, nil, msg.err)
	}
	return msg.runtime.wait
}

func (m *Model) handleSessionActivate(msg sessionActivateMsg) tea.Cmd {
	if !m.acceptsSessionRuntime(msg.id, msg.runtime) {
		return nil
	}
	if msg.runtime.ctx.Err() != nil {
		return nil
	}
	m.clearSessionSecurityInput()
	command := &sessionJoinCommand{runtime: msg.runtime, ready: msg.ready}
	return tea.Exec(command, func(execErr error) tea.Msg {
		return sessionJoinedMsg{id: msg.id, runtime: msg.runtime, err: execErr}
	})
}

func (m *Model) handleSecurityResize() tea.Cmd {
	if m.securityInput == nil {
		return nil
	}
	wasSuspended := m.securityInput.suspended
	state := m.securityInput.resize(m.width, m.height)
	m.securityInput = &state
	if state.kind == securityInputSecret && wasSuspended && !state.suspended && !m.helpVisible() {
		if m.sessionSecret != nil {
			return m.startSessionSecretInput()
		}
		return m.startCredentialSecretInput()
	}
	return nil
}

func (m *Model) handleSecurityInputKey(msg tea.KeyPressMsg) tea.Cmd {
	state := m.securityInput
	if state == nil {
		return nil
	}
	if m.helpVisible() {
		_, command := m.handleModalKey(msg)
		if !m.helpVisible() && state.kind == securityInputSecret && !state.suspended {
			if m.sessionSecret != nil {
				return m.startSessionSecretInput()
			}
			return m.startCredentialSecretInput()
		}
		return command
	}
	if state.suspended {
		switch {
		case key.Matches(msg, m.keys.Help):
			m.toggleHelp()
		case key.Matches(msg, m.keys.Quit):
			m.cancelCurrentOperation(true)
		}
		return nil
	}
	if state.kind != securityInputTrust || state.trust == nil {
		return nil
	}
	if key.Matches(msg, m.keys.Help, m.keys.FormHelp) {
		m.toggleHelp()
		return nil
	}
	pendingRuntime := m.sessionRuntime
	if pendingRuntime == nil {
		return nil
	}
	decision := app.TrustReject
	done := false
	switch {
	case msg.String() == "enter":
		done = true
	case key.Matches(msg, m.keys.Back):
		done = true
		m.cancelCurrentOperation(false)
	case key.Matches(msg, m.keys.Quit):
		done = true
		m.cancelCurrentOperation(true)
	default:
		decision, done = state.trust.update(msg)
	}
	if !done {
		return nil
	}
	request := sessionTrustRequestMsg{id: pendingRuntime.id, runtime: pendingRuntime}
	// The response channel is retained by the runtime request, not the display
	// state, so recover it from the active runtime handshake.
	if m.sessionTrustResponse == nil {
		return nil
	}
	request.response = m.sessionTrustResponse
	m.clearSessionSecurityInput()
	if pendingRuntime.ctx.Err() != nil {
		return pendingRuntime.wait
	}
	return m.respondTrust(request, decision, nil)
}

func (m *Model) respondTrust(request sessionTrustRequestMsg, decision app.TrustDecision, err error) tea.Cmd {
	return func() tea.Msg {
		select {
		case request.response <- sessionTrustResponse{decision: decision, err: err}:
		case <-request.runtime.ctx.Done():
		}
		return request.runtime.wait()
	}
}

func (m *Model) respondSecret(request sessionSecretRequestMsg, secret []byte, err error) tea.Cmd {
	return func() tea.Msg {
		select {
		case request.response <- sessionSecretResponse{secret: secret, err: err}:
		case <-request.runtime.ctx.Done():
			wipeSecret(secret)
		}
		return request.runtime.wait()
	}
}

func (m *Model) clearSessionSecurityInput() {
	if m.securityInput != nil && m.securityInput.secret != nil {
		m.securityInput.secret.clear()
	}
	m.securityInput = nil
	m.sessionSecret = nil
	m.sessionTrustResponse = nil
	m.credentialInput = nil
}

func (m *Model) sessionCommand(connection app.Connection) tea.Cmd {
	return m.sessionCommandWithRecovery(connection, nil)
}

func (m *Model) sessionCommandWithRecovery(connection app.Connection, recovery *errorModal) tea.Cmd {
	target := m.captureConnectionTarget(connection)
	id, operationCtx, ok := m.beginOperationWith(asyncOperationSSHStart, &target, operationOwnerModal, sshStartRetryIntent(connection))
	if !ok {
		return nil
	}
	attempt := app.SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port}
	var recoveryAttempt app.SSHAttemptTarget
	var recoveryFailure *app.SSHFailurePresentation
	if recovery != nil && recovery.failure != nil {
		recoveryAttempt = recovery.attempt
		copy := *recovery.failure
		recoveryFailure = &copy
	}
	expected := connection.Revision
	runtime := &sessionRuntime{
		id: id, ctx: operationCtx, service: m.connect,
		request:  app.ConnectRequest{Connection: app.ItemSelector{ID: connection.ID}, Expected: &expected},
		requests: make(chan tea.Msg, 1), done: make(chan struct{}), attempt: attempt,
		recoveryAttempt: recoveryAttempt, recoveryFailure: recoveryFailure,
	}
	m.sessionRuntime = runtime
	return tea.Batch(runtime.run, runtime.wait)
}

func (m *Model) handleSession(msg sessionFinishedMsg) tea.Cmd {
	if !m.acceptsOperationResult(msg.id, asyncOperationSSHStart) {
		return nil
	}
	m.clearSessionSecurityInput()
	m.sessionRuntime = nil
	var operationTarget *capturedTarget
	var retry *operationRetryIntent
	if m.operation != nil && m.operation.target != nil {
		copy := m.operation.target.clone()
		operationTarget = &copy
	}
	if m.operation != nil && m.operation.retry != nil {
		copy := m.operation.retry.clone()
		retry = &copy
	}
	resultKind := app.ErrorKindOf(msg.err)
	resultConflict := resultKind == app.ErrorKindNotFound || resultKind == app.ErrorKindConflict
	canceledBeforeCommit := m.operation != nil && m.operation.stopReason == operationStopCanceled && !sessionWasActive(msg.result) && !resultConflict
	commit := any(nil)
	if sessionWasActive(msg.result) {
		commit = msg.result
	}
	quit := m.completeOperation(msg.id, commit)
	m.status = "READY"
	if canceledBeforeCommit {
		if quit {
			return tea.Quit
		}
		return nil
	}
	preActive := !sessionWasActive(msg.result)
	if preActive && msg.result.Session.Failure != nil {
		if m.modal.kind == modalKindConnectConfirmation {
			m.closeGenericModal()
		}
		m.installSSHFailure(newSSHFailureModal(sessionAttempt(msg), *msg.result.Session.Failure))
		m.status = "ERROR"
		if quit {
			return tea.Quit
		}
		return nil
	}
	if msg.err != nil && preActive {
		if errors.Is(msg.err, context.Canceled) {
			if quit {
				return tea.Quit
			}
			return nil
		}
		if msg.recoveryFailure != nil {
			switch app.ErrorKindOf(msg.err) {
			case app.ErrorKindNotFound:
				failure := newSSHFailureModal(msg.recoveryAttempt, *msg.recoveryFailure)
				failure.recovery = recoveryMissing
				m.installSSHFailure(failure)
				m.status = "ERROR"
				return nil
			case app.ErrorKindConflict:
				failure := newSSHFailureModal(msg.recoveryAttempt, *msg.recoveryFailure)
				failure.recovery = recoveryConflict
				m.installSSHFailure(failure)
				m.status = "ERROR"
				return nil
			}
		}
		target := ""
		if operationTarget != nil {
			target = operationTarget.path
		}
		if target == "" {
			target = msg.result.Connection.Path
		}
		if target == "" {
			target = targetOf(m.browser.selection())
		}
		failure := newErrorModal("start SSH session", target, msg.err)
		failure.retry = retry
		attempt := sessionAttempt(msg)
		captured := capturedTarget{id: attempt.ID, revision: attempt.Revision, kind: app.NodeKindConnection, path: attempt.Path}
		if attempt.Host != "" && attempt.Port != 0 {
			captured.endpointOrScope = net.JoinHostPort(attempt.Host, strconv.Itoa(int(attempt.Port)))
		}
		if operationTarget != nil {
			captured = operationTarget.clone()
		}
		if !resultConflict || m.modal.kind != modalKindConnectConfirmation {
			if m.modal.kind == modalKindConnectConfirmation {
				opener := m.modal.openedFrom
				m.closeGenericModal()
				m.focusOwner = opener
			}
			m.openGenericModal(modalKindOperationError, &captured, operationErrorPayload{modal: failure})
		}
		m.status = "ERROR"
		if kind := resultKind; kind == app.ErrorKindConflict || kind == app.ErrorKindNotFound {
			conflictKind := conflictTypeRevisionChanged
			if kind == app.ErrorKindNotFound {
				conflictKind = conflictTypeMissing
			}
			if conflict, ok := newConflictState(conflictKind, captured, conflictOwnerModal); ok {
				m.modal.conflict = &conflict
				m.status = "CONFLICT"
			}
		}
		return nil
	}
	if msg.err == nil && msg.result.Session.State == app.SessionFailed && preActive {
		fallback := app.NewSSHStartError(app.SSHFailureUnexpected, app.SSHFailureStageUnknown, "", nil).Presentation()
		if m.modal.kind == modalKindConnectConfirmation {
			m.closeGenericModal()
		}
		m.installSSHFailure(newSSHFailureModal(sessionAttempt(msg), fallback))
		m.status = "ERROR"
		return nil
	}
	if m.modal.kind == modalKindConnectConfirmation {
		m.closeGenericModal()
	}
	m.sessionResult, m.sessionErr = msg.result, msg.err
	return tea.Quit
}

func (m *Model) installSSHFailure(failure *errorModal) {
	attempt := failure.attempt
	target := capturedTarget{id: attempt.ID, revision: attempt.Revision, kind: app.NodeKindConnection, path: attempt.Path}
	if attempt.Host != "" && attempt.Port != 0 {
		target.endpointOrScope = net.JoinHostPort(attempt.Host, strconv.Itoa(int(attempt.Port)))
	}
	if m.modal.kind == modalKindSSHFailure {
		m.modal.payload = sshFailurePayload{modal: failure}
		m.modal.target = &target
		m.modal.viewport = newViewportState(0)
		return
	}
	m.openGenericModal(modalKindSSHFailure, &target, sshFailurePayload{modal: failure})
}

func sessionAttempt(msg sessionFinishedMsg) app.SSHAttemptTarget {
	if msg.attempt.ID != "" || msg.attempt.Path != "" {
		return msg.attempt
	}
	return msg.result.Attempt
}

type credentialMutationCommand struct {
	ctx            context.Context
	terminal       app.Terminal
	service        ConnectionService
	folders        FolderService
	destination    *app.Folder
	create         *app.CreateConnectionRequest
	update         *app.UpdateConnectionRequest
	result         app.ConnectionResult
	err            error
	targetConflict bool
	input          io.Reader
	output         io.Writer
	errOut         io.Writer
	done           chan struct{}
	startGate      chan struct{}
}

type credentialMutationReadyMsg struct {
	id      uint64
	kind    operationKind
	command *credentialMutationCommand
	err     error
	discard bool
}

type credentialSecretFinishedMsg struct {
	id        uint64
	kind      operationKind
	command   *credentialMutationCommand
	suspended bool
	size      terminal.Size
	retry     bool
	err       error
}

func (c *credentialMutationCommand) SetStdin(input io.Reader)   { c.input = input }
func (c *credentialMutationCommand) SetStdout(output io.Writer) { c.output = output }
func (c *credentialMutationCommand) SetStderr(output io.Writer) { c.errOut = output }

func (c *credentialMutationCommand) Run() error {
	if c.terminal == nil || c.service == nil {
		c.err = errUnavailable
		return c.err
	}
	if err := verifyDestination(c.ctx, c.folders, c.destination); err != nil {
		c.err = err
		return err
	}
	secret, err := c.terminal.ReadSecret(c.ctx, terminal.SecretPrompt{Message: "Password: "})
	if err != nil {
		wipeSecret(secret)
		c.err = err
		return err
	}
	defer wipeSecret(secret)
	if c.create != nil {
		request := *c.create
		request.CredentialIntent = app.CredentialRemember
		request.Password = secret
		c.result, c.err = c.service.Create(c.ctx, request)
		c.targetConflict = destinationChangedAfterCreate(c.ctx, c.folders, c.destination, c.err)
		return c.err
	}
	if c.update != nil {
		request := *c.update
		request.CredentialIntent = app.CredentialRemember
		request.Password = secret
		c.result, c.err = c.service.Update(c.ctx, request)
		return c.err
	}
	c.err = errUnavailable
	return c.err
}

func (c *credentialMutationCommand) prepare(id uint64, kind operationKind) tea.Cmd {
	return func() tea.Msg {
		if c.terminal == nil || c.service == nil {
			return credentialMutationReadyMsg{id: id, kind: kind, command: c, err: errUnavailable}
		}
		err := verifyDestination(c.ctx, c.folders, c.destination)
		return credentialMutationReadyMsg{id: id, kind: kind, command: c, err: err}
	}
}

func (c *credentialMutationCommand) start(secret []byte) {
	c.done = make(chan struct{})
	c.startGate = make(chan struct{})
	startGate := c.startGate
	go func() {
		defer close(c.done)
		defer wipeSecret(secret)
		select {
		case <-c.ctx.Done():
			c.err = c.ctx.Err()
			return
		case <-startGate:
		}
		if c.create != nil {
			request := *c.create
			request.CredentialIntent = app.CredentialRemember
			request.Password = secret
			c.result, c.err = c.service.Create(c.ctx, request)
			c.targetConflict = destinationChangedAfterCreate(c.ctx, c.folders, c.destination, c.err)
			return
		}
		if c.update != nil {
			request := *c.update
			request.CredentialIntent = app.CredentialRemember
			request.Password = secret
			c.result, c.err = c.service.Update(c.ctx, request)
			return
		}
		c.err = errUnavailable
	}()
}

func (c *credentialMutationCommand) release() {
	if c.startGate != nil {
		close(c.startGate)
		c.startGate = nil
	}
}

func (c *credentialMutationCommand) wait(id uint64, kind operationKind) tea.Cmd {
	return func() tea.Msg {
		<-c.done
		return operationResultMsg{id: id, kind: kind, targetConflict: c.targetConflict, connection: c.result, err: c.err}
	}
}

func (m *Model) handleCredentialMutationReady(msg credentialMutationReadyMsg) tea.Cmd {
	if !m.acceptsOperationResult(msg.id, asyncOperationSave) {
		return nil
	}
	if m.operation == nil || m.operation.ctx == nil {
		return nil
	}
	if err := m.operation.ctx.Err(); err != nil {
		return func() tea.Msg {
			return operationResultMsg{id: msg.id, kind: msg.kind, err: err}
		}
	}
	if msg.err != nil {
		return func() tea.Msg {
			return operationResultMsg{id: msg.id, kind: msg.kind, targetConflict: msg.kind == operationCreate, err: msg.err}
		}
	}
	state, ok := newSecurityInputState(securityInputSecret, m.focusOwner)
	if !ok {
		return func() tea.Msg {
			return operationResultMsg{id: msg.id, kind: msg.kind, err: errUnavailable}
		}
	}
	state = state.resize(m.width, m.height)
	state.secret = newSecretPrompt(app.SecretPassword, "")
	m.securityInput = &state
	pending := msg
	m.credentialInput = &pending
	if state.suspended {
		return nil
	}
	return m.startCredentialSecretInput()
}

func (m *Model) startCredentialSecretInput() tea.Cmd {
	if m.securityInput == nil || m.securityInput.suspended || m.credentialInput == nil || m.operation == nil || m.operation.ctx == nil {
		return nil
	}
	pending := *m.credentialInput
	command := &gatedSecretCommand{
		ctx: m.operation.ctx, terminal: m.terminal,
		prompt: terminal.SecretPrompt{Message: "Password: "}, discard: pending.discard,
		consume: func(secret []byte, err error) {
			if err != nil {
				pending.command.err = err
				return
			}
			pending.command.start(secret)
		},
	}
	return tea.Exec(command, func(execErr error) tea.Msg {
		if command.err == nil && execErr != nil {
			command.err = execErr
		}
		return credentialSecretFinishedMsg{
			id: pending.id, kind: pending.kind, command: pending.command,
			suspended: command.suspended, size: command.size, retry: command.retry, err: command.err,
		}
	})
}

func (m *Model) handleCredentialSecretFinished(msg credentialSecretFinishedMsg) tea.Cmd {
	if !m.acceptsOperationResult(msg.id, asyncOperationSave) || m.credentialInput == nil || m.credentialInput.command != msg.command {
		return nil
	}
	if msg.suspended {
		m.width, m.height = msg.size.Columns, msg.size.Rows
		state := m.securityInput.resize(m.width, m.height)
		m.securityInput = &state
		m.credentialInput.discard = true
		return nil
	}
	if msg.retry {
		m.credentialInput.discard = false
		return m.startCredentialSecretInput()
	}
	m.clearSessionSecurityInput()
	if msg.command.done != nil {
		msg.command.release()
		return msg.command.wait(msg.id, msg.kind)
	}
	if errors.Is(msg.err, terminal.ErrSecretQuit) {
		m.cancelCurrentOperation(true)
		msg.err = context.Canceled
	} else if errors.Is(msg.err, context.Canceled) {
		m.cancelCurrentOperation(false)
	}
	return func() tea.Msg {
		return operationResultMsg{id: msg.id, kind: msg.kind, err: msg.err}
	}
}

func wipeSecret(secret []byte) {
	for index := range secret {
		secret[index] = 0
	}
}

func sessionWasActive(result app.ConnectResult) bool {
	return !result.Session.StartedAt.Equal(time.Time{}) || result.Session.RemoteExitStatus != nil
}
