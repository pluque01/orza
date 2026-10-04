package app

import (
	"context"
	"errors"
	"net"
	"sort"
	"strconv"
	"strings"
	"sync"
	"unicode"

	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/terminal"
)

// ParseTunnelEndpoint parses an endpoint without DNS resolution or shell syntax.
// An empty host is allowed here; only listening endpoints supply a default.
func ParseTunnelEndpoint(input string) (TunnelEndpoint, error) {
	host, port, err := net.SplitHostPort(input)
	if err != nil || port == "" || !validTunnelHost(host, true) {
		return TunnelEndpoint{}, safeUseCaseError("parse tunnel endpoint", "", ErrInvalidRequest)
	}
	// SplitHostPort also accepts brackets around names, which are not our grammar.
	if strings.HasPrefix(input, "[") && net.ParseIP(host) == nil {
		return TunnelEndpoint{}, safeUseCaseError("parse tunnel endpoint", "", ErrInvalidRequest)
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return TunnelEndpoint{}, safeUseCaseError("parse tunnel endpoint", "", ErrInvalidRequest)
		}
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return TunnelEndpoint{}, safeUseCaseError("parse tunnel endpoint", "", ErrInvalidRequest)
	}
	if ip := net.ParseIP(host); ip != nil {
		host = ip.String()
	}
	return TunnelEndpoint{Host: host, Port: uint16(n)}, nil
}

func validTunnelHost(host string, empty bool) bool {
	if host == "" {
		return empty
	}
	for _, r := range host {
		if unicode.IsSpace(r) || unicode.IsControl(r) || isBidiControl(r) || r == '[' || r == ']' {
			return false
		}
	}
	if net.ParseIP(host) != nil {
		return true
	}
	if len(host) > 253 {
		return false
	}
	for _, label := range strings.Split(strings.TrimSuffix(host, "."), ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, r := range label {
			if !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') {
				return false
			}
		}
	}
	return true
}

// ValidateTunnelConfig returns the canonical confirmed request. Destination
// names remain unresolved so the forwarding mode determines the DNS machine.
func ValidateTunnelConfig(config TunnelConfig) (TunnelConfig, error) {
	invalid := func() (TunnelConfig, error) {
		return TunnelConfig{}, safeUseCaseError("validate tunnel", "", ErrInvalidRequest)
	}
	switch config.Mode {
	case TunnelLocal, TunnelRemote, TunnelDynamic:
	default:
		return invalid()
	}
	if config.Listen.Port == 0 {
		return invalid()
	}
	if config.Listen.Host == "" || strings.EqualFold(config.Listen.Host, "localhost") {
		config.Listen.Host = "127.0.0.1"
	}
	ip := net.ParseIP(config.Listen.Host)
	if ip == nil {
		return invalid()
	}
	config.Listen.Host = ip.String()
	if !ip.IsLoopback() && !config.ExposureAcknowledged {
		return invalid()
	}
	if config.Mode == TunnelDynamic {
		if config.Destination != (TunnelEndpoint{}) {
			return invalid()
		}
	} else {
		if config.Destination.Port == 0 || !validTunnelHost(config.Destination.Host, false) {
			return invalid()
		}
		if ip := net.ParseIP(config.Destination.Host); ip != nil {
			config.Destination.Host = ip.String()
		}
	}
	return config, nil
}

type TunnelOptions struct {
	Connections  ConnectionRepository
	Credentials  CredentialLifecycle
	HostTrust    HostTrust
	TrustedHosts TrustedHostRepository
	Store        CredentialStore
	Scope        credential.Scope
	Runner       SSHTunnelRunner
	Terminal     Terminal
}

type tunnelEntry struct {
	snapshot  TunnelSnapshot
	cancel    context.CancelFunc
	done      chan struct{}
	err       error
	completed uint64
}

// TunnelService owns a session root independent of short-lived UI operations.
// RunTunnel returns only after the adapter's owned resources have converged.
type TunnelService struct {
	options        TunnelOptions
	root           context.Context
	cancel         context.CancelFunc
	mu             sync.Mutex
	recoveryMu     sync.Mutex
	entries        map[uint64]*tunnelEntry
	nextID         uint64
	nextAttempt    uint64
	nextCompletion uint64
	closed         bool
	closeOnce      sync.Once
}

func NewTunnelService(opts TunnelOptions) (*TunnelService, error) {
	if opts.Connections == nil || opts.Credentials == nil || opts.HostTrust == nil || opts.TrustedHosts == nil || opts.Store == nil || opts.Scope == "" || opts.Runner == nil {
		return nil, safeUseCaseError("configure tunnel service", "", ErrInvalidRequest)
	}
	root, cancel := context.WithCancel(context.Background())
	return &TunnelService{options: opts, root: root, cancel: cancel, entries: make(map[uint64]*tunnelEntry)}, nil
}

func (s *TunnelService) Start(ctx context.Context, request TunnelRequest) (TunnelSnapshot, error) {
	return s.start(ctx, 0, request)
}

func (s *TunnelService) Retry(ctx context.Context, id uint64, request TunnelRequest) (TunnelSnapshot, error) {
	if id == 0 {
		return TunnelSnapshot{}, safeUseCaseError("retry tunnel", "", ErrNotFound)
	}
	return s.start(ctx, id, request)
}

func (s *TunnelService) start(ctx context.Context, id uint64, request TunnelRequest) (TunnelSnapshot, error) {
	target := selectorTarget(request.Connection)
	fail := func(err error) (TunnelSnapshot, error) {
		return TunnelSnapshot{}, safeUseCaseError("start tunnel", target, err)
	}
	if err := contextError(ctx); err != nil {
		return fail(err)
	}
	if err := validateSelector(request.Connection); err != nil {
		return fail(ErrInvalidRequest)
	}
	// Presentation resolves selectors before confirmation. Only the captured
	// stable ID and revision may authorize network access, including on retry.
	if request.Connection.ID == "" || request.Expected == nil || *request.Expected == 0 {
		return fail(ErrInvalidRequest)
	}
	rev := *request.Expected
	request.Expected = &rev
	if request.NonInteractive && (request.DecideTrust != nil || request.ReadSecret != nil) {
		return fail(ErrInvalidRequest)
	}
	config, err := ValidateTunnelConfig(request.Config)
	if err != nil {
		return fail(err)
	}
	// Preflight before repository access; repeat under lock after resolving.
	s.mu.Lock()
	err = s.admitLocked(id, request.Connection)
	s.mu.Unlock()
	if err != nil {
		return fail(err)
	}
	resolved, err := s.options.Connections.GetConnection(ctx, request.Connection)
	if err != nil {
		return fail(err)
	}
	connection := resolved.Connection
	if connection.ID == "" || connection.Revision == 0 {
		return fail(ErrInvalidRequest)
	}
	if connection.ID != request.Connection.ID || *request.Expected != connection.Revision {
		return fail(ErrConflict)
	}
	if err := contextError(ctx); err != nil {
		return fail(err)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.admitLocked(id, ItemSelector{ID: connection.ID}); err != nil {
		return fail(err)
	}
	version := uint64(1)
	if id == 0 {
		s.nextID++
		id = s.nextID
	} else {
		version = s.entries[id].snapshot.Version + 1
	}
	s.nextAttempt++
	runCtx, cancel := context.WithCancel(s.root)
	entry := &tunnelEntry{snapshot: TunnelSnapshot{
		ID: id, Attempt: s.nextAttempt, Version: version, State: TunnelStarting, Config: config,
		Connection: SSHAttemptTarget{ID: connection.ID, Revision: connection.Revision, Path: connection.Path, Host: connection.Host, Port: connection.Port},
	}, cancel: cancel, done: make(chan struct{})}
	if config.Mode == TunnelRemote {
		entry.snapshot.Warning = "Requested remote listener scope is unverified; server release after transport loss is not verified."
	}
	if !net.ParseIP(config.Listen.Host).IsLoopback() {
		if entry.snapshot.Warning != "" {
			entry.snapshot.Warning += " "
		}
		entry.snapshot.Warning += "Non-loopback listener permits external clients."
	}
	if config.Mode == TunnelDynamic {
		if entry.snapshot.Warning != "" {
			entry.snapshot.Warning += " "
		}
		entry.snapshot.Warning += "SOCKS5 proxy clients are not authenticated."
	}
	s.entries[id] = entry
	go s.run(runCtx, entry, connection, request)
	return entry.snapshot, nil
}

func (s *TunnelService) admitLocked(id uint64, selector ItemSelector) error {
	if s.closed {
		return ErrConflict
	}
	if id != 0 {
		entry, ok := s.entries[id]
		if !ok {
			return ErrNotFound
		}
		if entry.snapshot.Live() || selector.ID != entry.snapshot.Connection.ID || selector.Path != "" {
			return ErrConflict
		}
	}
	live := 0
	for _, entry := range s.entries {
		if entry.snapshot.Live() {
			live++
		}
	}
	if live >= 16 {
		return ErrConflict
	}
	return nil
}

func (s *TunnelService) run(ctx context.Context, entry *tunnelEntry, connection Connection, request TunnelRequest) {
	gate := &hostVerificationGate{connection: connection, trust: s.options.HostTrust, trustedHosts: s.options.TrustedHosts, decide: request.DecideTrust}
	secret := func(ctx context.Context, req SecretRequest) ([]byte, error) {
		if !gate.verifiedHost() {
			return nil, safeUseCaseError("request authentication secret", connection.Path, ErrHostNotVerified)
		}
		if err := contextError(ctx); err != nil {
			return nil, NormalizeSSHStartError(SSHFailureStageCredential, err)
		}
		unavailable := func(cause error) ([]byte, error) {
			return nil, NewSSHStartError(SSHFailureCredentialUnavailable, SSHFailureStageCredential, "secure store", cause)
		}
		switch req.Kind {
		case SecretPassword:
			if connection.AuthMethod != AuthMethodPassword {
				return nil, safeUseCaseError("request authentication secret", connection.Path, ErrInvalidRequest)
			}
			if connection.CredentialRef != "" {
				stored, err := s.options.Store.Get(ctx, credential.Key{Scope: s.options.Scope, Reference: credential.Reference(connection.CredentialRef)})
				if err == nil {
					return stored, nil
				}
				wipeSecret(stored)
				if request.NonInteractive || !errors.Is(err, credential.ErrNotFound) && !errors.Is(err, credential.ErrUnavailable) {
					return unavailable(err)
				}
			}
		case SecretPassphrase:
			if connection.AuthMethod != AuthMethodKey {
				return nil, safeUseCaseError("request authentication secret", connection.Path, ErrInvalidRequest)
			}
		default:
			return nil, safeUseCaseError("request authentication secret", connection.Path, ErrInvalidRequest)
		}
		if request.NonInteractive {
			return unavailable(credential.ErrUnavailable)
		}
		if request.ReadSecret != nil {
			secret, err := request.ReadSecret(ctx, req)
			if err != nil {
				wipeSecret(secret)
				return nil, terminalSecretError(connection.Path, err)
			}
			return secret, nil
		}
		if s.options.Terminal == nil || !s.options.Terminal.Interactive() {
			return unavailable(ErrNonInteractive)
		}
		prompt := "Password"
		if req.Kind == SecretPassphrase {
			prompt = "Private key passphrase"
		}
		secret, err := s.options.Terminal.ReadSecret(ctx, terminal.SecretPrompt{Message: prompt})
		if err != nil {
			wipeSecret(secret)
			return nil, terminalSecretError(connection.Path, err)
		}
		return secret, nil
	}
	// Recovery may mutate shared saga state; do not run multiple recoveries at once.
	s.recoveryMu.Lock()
	err := contextError(ctx)
	if err == nil {
		err = s.options.Credentials.Recover(ctx)
	}
	s.recoveryMu.Unlock()
	if err != nil {
		err = normalizeConnectFailure(SSHFailureStageCredential, err)
	} else {
		err = s.options.Runner.RunTunnel(ctx, TunnelRunRequest{Connection: connection, Config: entry.snapshot.Config, VerifyHost: gate.verify, Secret: secret,
			Ready: func() {
				s.mu.Lock()
				defer s.mu.Unlock()
				if s.entries[entry.snapshot.ID] != entry || ctx.Err() != nil || entry.snapshot.State != TunnelStarting {
					return
				}
				entry.snapshot.State = TunnelActive
				entry.snapshot.Scope = "local_bound"
				if entry.snapshot.Config.Mode == TunnelRemote {
					entry.snapshot.Scope = "unverified"
				}
				entry.snapshot.Version++
			},
			Diagnostic: func(text string) {
				text = controlledTunnelDiagnostic(text)
				if text == "" {
					return
				}
				s.mu.Lock()
				defer s.mu.Unlock()
				if s.entries[entry.snapshot.ID] != entry || ctx.Err() != nil || !entry.snapshot.Live() || entry.snapshot.State == TunnelStopping || entry.snapshot.Diagnostic == text {
					return
				}
				entry.snapshot.Diagnostic = text
				entry.snapshot.Version++
			},
		})
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if ctx.Err() != nil {
		entry.snapshot.State = TunnelStopped
		entry.err = normalizeConnectFailure(SSHFailureStageUnknown, ctx.Err())
	} else if err != nil {
		entry.snapshot.State = TunnelFailed
		entry.err = normalizeConnectFailure(SSHFailureStageUnknown, err)
		var failure *SSHStartError
		if entry.snapshot.Diagnostic == "" && errors.As(entry.err, &failure) {
			entry.snapshot.Diagnostic = failure.Presentation().Summary
		}
	} else {
		entry.snapshot.State = TunnelStopped
	}
	entry.cancel()
	entry.snapshot.Version++
	s.nextCompletion++
	entry.completed = s.nextCompletion
	close(entry.done)
	s.evictLocked()
}

// Accept only controlled adapter categories, never sanitized arbitrary text.
func controlledTunnelDiagnostic(text string) string {
	switch text {
	case "binding", "trust", "authentication", "forwarding_policy", "destination", "transport", "canceled", "capacity", "unexpected",
		"listener unavailable", "forwarding request refused", "destination unavailable", "destination timed out", "client limit reached", "transport lost", "proxy request rejected", "protocol convergence failed",
		"Forwarding listener unavailable.", "Forwarding client capacity reached.", "Proxy request rejected.", "Forwarding destination unavailable.",
		"Forwarding transport lost.", "Forwarding protocol cleanup stalled.":
		return text
	default:
		return ""
	}
}

func (s *TunnelService) evictLocked() {
	var terminal []*tunnelEntry
	for _, entry := range s.entries {
		if !entry.snapshot.Live() {
			terminal = append(terminal, entry)
		}
	}
	sort.Slice(terminal, func(i, j int) bool { return terminal[i].completed < terminal[j].completed })
	for len(terminal) > 32 {
		delete(s.entries, terminal[0].snapshot.ID)
		terminal = terminal[1:]
	}
}

func (s *TunnelService) Stop(ctx context.Context, id uint64) error {
	if err := contextError(ctx); err != nil {
		return safeUseCaseError("stop tunnel", "", err)
	}
	s.mu.Lock()
	entry, ok := s.entries[id]
	if !ok {
		s.mu.Unlock()
		return safeUseCaseError("stop tunnel", "", ErrNotFound)
	}
	s.stopLocked(entry)
	s.mu.Unlock()
	// Cancellation has transferred cleanup ownership to Stop. Returning early
	// here would allow a caller to exit while the listener was still alive.
	<-entry.done
	return nil
}

// StopAttempt cleans up an operation's captured attempt without stopping a
// later retry. Missing entries and stale attempt tokens are harmless no-ops.
func (s *TunnelService) StopAttempt(ctx context.Context, id, attempt uint64) error {
	if err := contextError(ctx); err != nil {
		return safeUseCaseError("stop tunnel attempt", "", err)
	}
	s.mu.Lock()
	entry, ok := s.entries[id]
	if !ok || entry.snapshot.Attempt != attempt {
		s.mu.Unlock()
		return nil
	}
	s.stopLocked(entry)
	s.mu.Unlock()
	// Join the captured entry, never a subsequent entry with the same tunnel ID.
	<-entry.done
	return nil
}

func (s *TunnelService) stopLocked(entry *tunnelEntry) {
	if entry.snapshot.Live() && entry.snapshot.State != TunnelStopping {
		entry.snapshot.State = TunnelStopping
		entry.snapshot.Version++
		entry.cancel()
	}
}

func (s *TunnelService) Wait(ctx context.Context, id uint64) error {
	if err := contextError(ctx); err != nil {
		return safeUseCaseError("wait for tunnel", "", err)
	}
	s.mu.Lock()
	entry, ok := s.entries[id]
	s.mu.Unlock()
	if !ok {
		return safeUseCaseError("wait for tunnel", "", ErrNotFound)
	}
	select {
	case <-entry.done:
		if entry.err != nil {
			return safeUseCaseError("run tunnel", entry.snapshot.Connection.Path, entry.err)
		}
		return nil
	case <-ctx.Done():
		return safeUseCaseError("wait for tunnel", "", ctx.Err())
	}
}

func (s *TunnelService) Snapshots() []TunnelSnapshot {
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshots := make([]TunnelSnapshot, 0, len(s.entries))
	for _, entry := range s.entries {
		snapshots = append(snapshots, entry.snapshot)
	}
	sort.Slice(snapshots, func(i, j int) bool { return snapshots[i].ID < snapshots[j].ID })
	return snapshots
}

func (s *TunnelService) Get(id uint64) (TunnelSnapshot, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	if !ok {
		return TunnelSnapshot{}, false
	}
	return entry.snapshot, true
}

func (s *TunnelService) Dismiss(id uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.entries[id]
	if !ok {
		return safeUseCaseError("dismiss tunnel", "", ErrNotFound)
	}
	if entry.snapshot.Live() {
		return safeUseCaseError("dismiss tunnel", "", ErrConflict)
	}
	delete(s.entries, id)
	return nil
}

func (s *TunnelService) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		s.mu.Lock()
		s.closed = true
		done := make([]<-chan struct{}, 0, len(s.entries))
		for _, entry := range s.entries {
			s.stopLocked(entry)
			done = append(done, entry.done)
		}
		s.cancel()
		s.mu.Unlock()
		for _, ch := range done {
			<-ch
		}
	})
	return nil
}
