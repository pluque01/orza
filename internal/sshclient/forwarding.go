package sshclient

import (
	"context"
	"errors"
	"io"
	"net"
	"sync"
	"time"

	"github.com/pluque01/orza/internal/app"
)

// Separate from sshTransport so session-only test transports remain valid.
type forwardingTransport interface {
	Dial(string, string) (net.Conn, error)
	Listen(string, string) (net.Listener, error)
	Wait() error
}

func (t *clientTransport) Dial(network, address string) (net.Conn, error) {
	return t.client.Dial(network, address)
}
func (t *clientTransport) Listen(network, address string) (net.Listener, error) {
	listener, err := t.client.Listen(network, address)
	if err != nil {
		return nil, err
	}
	return &remoteForwardListener{Listener: listener, closed: make(chan struct{})}, nil
}
func (t *clientTransport) Wait() error { return t.client.Wait() }

type remoteForwardListener struct {
	net.Listener
	closed   chan struct{}
	once     sync.Once
	closeErr error
}

func (l *remoteForwardListener) Close() error {
	l.once.Do(func() { l.closeErr = l.Listener.Close(); close(l.closed) })
	return l.closeErr
}

func (l *remoteForwardListener) Accept() (net.Conn, error) {
	for {
		conn, err := l.Listener.Accept()
		if err == nil {
			return conn, nil
		}
		// A failed channel confirmation still consumed a queue entry. Continue
		// draining until listener cleanup completes, otherwise its
		// forward-list lock can prevent listener/transport shutdown forever.
		// Transport Wait supplies loss detection independently of acceptance.
		if err == io.EOF {
			// EOF is ambiguous: it can also come from a failed confirmation,
			// not just the closed queue. Retry while Close resolves bookkeeping.
			select {
			case <-l.closed:
				return nil, io.EOF
			case <-time.After(time.Millisecond):
			}
		}
	}
}

var errTunnel = errors.New("SSH forwarding transport failed")

type forwardingTimer interface{ Stop() bool }
type forwardingAfterFunc func(time.Duration, func()) forwardingTimer

type tunnelRuntime struct {
	ctx            context.Context
	cancel         context.CancelCauseFunc
	raw            *closeOnce
	stopOnce       sync.Once
	watchdogDone   chan struct{}
	converged      chan struct{}
	mu             sync.Mutex
	connections    map[*closeOnce]struct{}
	diagnostic     func(string)
	lastDiagnostic string
	afterFunc      forwardingAfterFunc
}

// The returned function cancels a pending timer or joins its running callback.
func (r *tunnelRuntime) deadline(delay time.Duration, expire func()) func() {
	afterFunc := r.afterFunc
	if afterFunc == nil {
		afterFunc = func(d time.Duration, f func()) forwardingTimer { return time.AfterFunc(d, f) }
	}
	joined := make(chan struct{})
	timer := afterFunc(delay, func() { defer close(joined); expire() })
	return func() {
		if timer.Stop() {
			close(joined)
		}
		<-joined
	}
}

func (r *tunnelRuntime) stop(cause error) {
	r.stopOnce.Do(func() {
		// Arm independently BEFORE cancel makes any protocol cleanup runnable.
		finish := r.deadline(250*time.Millisecond, func() { _ = r.raw.Close() })
		go func() {
			defer close(r.watchdogDone)
			<-r.converged
			finish()
		}()
		r.cancel(cause)
	})
}
func (r *tunnelRuntime) report(text string) {
	switch text {
	case "Forwarding destination unavailable.", "Forwarding listener unavailable.", "Proxy request rejected.", "Forwarding client capacity reached.", "Forwarding transport lost.", "Forwarding protocol cleanup stalled.", "Remote listener accepted; requested scope is unverified.":
	default:
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if text == r.lastDiagnostic {
		return
	}
	r.lastDiagnostic = text
	if r.diagnostic != nil {
		r.diagnostic(text)
	}
}

// A stalled protocol write is unhealthy, unlike an ordinary refused destination.
// The timer worker is joined even when Stop races its callback.
func (r *tunnelRuntime) protocolCall(call func() error) error {
	finish := r.deadline(time.Second, func() {
		r.stop(errTunnel)
		r.report("Forwarding protocol cleanup stalled.")
	})
	defer finish()
	return call()
}
func (r *tunnelRuntime) track(conn net.Conn) *closeOnce {
	closer := newCloseOnce(conn)
	if closer == nil {
		return nil
	}
	r.mu.Lock()
	r.connections[closer] = struct{}{}
	stopping := r.ctx.Err() != nil
	r.mu.Unlock()
	if stopping {
		_ = r.protocolCall(closer.Close)
	}
	return closer
}
func (r *tunnelRuntime) release(closer *closeOnce) {
	if closer == nil {
		return
	}
	_ = r.protocolCall(closer.Close)
	r.mu.Lock()
	delete(r.connections, closer)
	r.mu.Unlock()
}

// RunTunnel owns one authenticated transport, with no session channel or terminal.
func (c *Client) RunTunnel(ctx context.Context, request app.TunnelRunRequest) (runErr error) {
	if ctx == nil {
		return stageError(StageSession, errTunnel)
	}
	if ctx.Err() != nil {
		return ctx.Err()
	}
	mode := request.Config.Mode
	if request.VerifyHost == nil || request.Connection.Host == "" || request.Connection.Port == 0 || request.Connection.Username == "" ||
		(mode != app.TunnelLocal && mode != app.TunnelRemote && mode != app.TunnelDynamic) || net.ParseIP(request.Config.Listen.Host) == nil || request.Config.Listen.Port == 0 ||
		(mode != app.TunnelDynamic && (request.Config.Destination.Host == "" || request.Config.Destination.Port == 0)) ||
		(!net.ParseIP(request.Config.Listen.Host).IsLoopback() && !request.Config.ExposureAcknowledged) {
		return stageError(StageSession, errTunnel)
	}
	resources := &resourceSet{}
	transport, err := c.openTransport(ctx, app.SSHSessionRequest{Connection: request.Connection, VerifyHost: request.VerifyHost, Secret: request.Secret}, resources)
	if err != nil {
		_ = resources.raw.Close()
		_ = resources.close()
		return err
	}
	forward, ok := transport.(forwardingTransport)
	if !ok {
		_ = resources.raw.Close()
		_ = resources.close()
		return stageError(StageHandshake, errTunnel)
	}
	lifetime, cancel := context.WithCancelCause(context.WithoutCancel(ctx))
	r := &tunnelRuntime{ctx: lifetime, cancel: cancel, raw: resources.raw, connections: make(map[*closeOnce]struct{}), diagnostic: request.Diagnostic, watchdogDone: make(chan struct{}), converged: make(chan struct{}), afterFunc: c.options.forwardAfterFunc}
	rootJoined := make(chan struct{})
	go func() {
		defer close(rootJoined)
		select {
		case <-ctx.Done():
			r.stop(ctx.Err())
		case <-lifetime.Done():
		}
	}()
	waitJoined := make(chan struct{})
	go func() {
		defer close(waitJoined)
		_ = forward.Wait()
		if lifetime.Err() == nil {
			r.report("Forwarding transport lost.")
		}
		r.stop(errTunnel)
	}()
	var listener net.Listener
	var acceptJoined chan struct{}
	var work sync.WaitGroup
	defer func() {
		r.stop(errTunnel)
		var cleanup sync.WaitGroup
		if listener != nil {
			cleanup.Add(1)
			go func() { defer cleanup.Done(); _ = r.protocolCall(listener.Close) }()
		}
		r.mu.Lock()
		connections := make([]*closeOnce, 0, len(r.connections))
		for conn := range r.connections {
			connections = append(connections, conn)
		}
		r.mu.Unlock()
		for _, conn := range connections {
			cleanup.Add(1)
			go func(conn *closeOnce) { defer cleanup.Done(); _ = r.protocolCall(conn.Close) }(conn)
		}
		if acceptJoined != nil {
			<-acceptJoined
		}
		work.Wait()
		cleanup.Wait()
		_ = r.protocolCall(resources.transport.Close)
		_ = resources.raw.Close()
		_ = resources.auth.Close()
		<-waitJoined
		<-rootJoined
		close(r.converged)
		<-r.watchdogDone
		if ctx.Err() != nil {
			runErr = ctx.Err()
		}
	}()
	if mode == app.TunnelRemote {
		type result struct {
			listener net.Listener
			err      error
		}
		reply := make(chan result, 1)
		joined := make(chan struct{})
		go func() {
			defer close(joined)
			l, e := forward.Listen("tcp", request.Config.Listen.String())
			reply <- result{l, e}
		}()
		select {
		case value := <-reply:
			<-joined
			listener, err = value.listener, value.err
		case <-lifetime.Done():
			value := <-reply
			<-joined
			listener = value.listener
			return context.Cause(lifetime)
		}
	} else {
		listener, err = (&net.ListenConfig{}).Listen(lifetime, "tcp", request.Config.Listen.String())
	}
	if err != nil || listener == nil {
		r.report("Forwarding listener unavailable.")
		return stageError(StageSession, errTunnel)
	}
	if ctx.Err() != nil {
		r.stop(ctx.Err())
	}
	if lifetime.Err() != nil {
		return context.Cause(lifetime)
	}
	if mode == app.TunnelRemote {
		r.report("Remote listener accepted; requested scope is unverified.")
	}
	if request.Ready != nil {
		request.Ready()
	}
	slots := make(chan struct{}, 64)
	acceptJoined = make(chan struct{})
	// Vendor acceptance precedes admission. One serialized overflow cleanup is
	// allowed beyond the 64 slots. Trusted servers may transiently buffer channels;
	// this is not a bound on malicious-server multiplexer internals.
	go func() {
		defer close(acceptJoined)
		for {
			incoming, e := listener.Accept()
			if e != nil {
				if incoming != nil {
					r.release(r.track(incoming))
				}
				if lifetime.Err() == nil {
					r.stop(errTunnel)
				}
				return
			}
			if incoming == nil {
				r.stop(errTunnel)
				return
			}
			source := r.track(incoming)
			if lifetime.Err() != nil {
				r.release(source)
				if mode != app.TunnelRemote {
					return
				}
				continue
			}
			select {
			case slots <- struct{}{}:
				work.Add(1)
				go func() {
					defer work.Done()
					defer func() { <-slots }()
					defer r.release(source)
					r.serve(forward, request.Config, incoming, source)
				}()
			default:
				r.report("Forwarding client capacity reached.")
				if mode == app.TunnelDynamic {
					_ = incoming.SetWriteDeadline(time.Now().Add(time.Second))
					_ = socksReply(incoming, 2)
				}
				r.release(source)
			}
		}
	}()
	<-lifetime.Done()
	return context.Cause(lifetime)
}

func (r *tunnelRuntime) serve(transport forwardingTransport, config app.TunnelConfig, source net.Conn, sourceCloser *closeOnce) {
	address := config.Destination.String()
	if config.Mode == app.TunnelDynamic {
		_ = source.SetDeadline(time.Now().Add(10 * time.Second))
		var code byte
		var err error
		address, code, err = socksRequest(source, source)
		if err != nil {
			if code != 0 {
				_ = socksReply(source, code)
			}
			r.report("Proxy request rejected.")
			return
		}
		// Negotiation and destination establishment have separate budgets.
		_ = source.SetDeadline(time.Now().Add(10 * time.Second))
	}
	ctx, cancel := context.WithTimeout(r.ctx, 10*time.Second)
	defer cancel()
	type result struct {
		conn net.Conn
		err  error
	}
	reply := make(chan result, 1)
	openJoined := make(chan struct{})
	go func() {
		defer close(openJoined)
		var conn net.Conn
		var err error
		if config.Mode == app.TunnelRemote {
			conn, err = (&net.Dialer{}).DialContext(ctx, "tcp", address)
		} else {
			conn, err = transport.Dial("tcp", address)
		}
		reply <- result{conn, err}
	}()
	var opened result
	select {
	case opened = <-reply:
		<-openJoined
	case <-ctx.Done():
		// Retain admission and own the late result until actual convergence.
		finish := r.deadline(time.Second, func() {
			r.stop(errTunnel)
			r.report("Forwarding protocol cleanup stalled.")
		})
		opened = <-reply
		<-openJoined
		finish()
		if opened.conn != nil {
			r.release(r.track(opened.conn))
		}
		if config.Mode == app.TunnelDynamic {
			_ = socksReply(source, 1)
		}
		r.report("Forwarding destination unavailable.")
		return
	}
	var destination *closeOnce
	if opened.conn != nil {
		destination = r.track(opened.conn)
		defer r.release(destination)
	}
	if opened.err != nil || opened.conn == nil || ctx.Err() != nil {
		if config.Mode == app.TunnelDynamic {
			_ = socksReply(source, 1)
		}
		r.report("Forwarding destination unavailable.")
		return
	}
	if config.Mode == app.TunnelDynamic {
		if err := socksReply(source, 0); err != nil {
			return
		}
		_ = source.SetDeadline(time.Time{})
	}
	r.copy(source, opened.conn, sourceCloser, destination)
}

func (r *tunnelRuntime) copy(a, b net.Conn, aCloser, bCloser *closeOnce) {
	results := make(chan error, 2)
	var workers sync.WaitGroup
	workers.Add(2)
	transfer := func(dst, src net.Conn) {
		defer workers.Done()
		_, err := io.CopyBuffer(dst, src, make([]byte, 32*1024))
		if err == nil {
			if half, ok := dst.(interface{ CloseWrite() error }); ok {
				err = r.protocolCall(half.CloseWrite)
			}
		}
		results <- err
	}
	go transfer(a, b)
	go transfer(b, a)
	first := <-results
	if first != nil {
		// Successful channel Close does not guarantee the peer unblocks Read.
		// Keep the error-cleanup deadline armed until both copies are joined.
		finish := r.deadline(time.Second, func() {
			r.stop(errTunnel)
			r.report("Forwarding protocol cleanup stalled.")
		})
		defer finish()
		var closeWorkers sync.WaitGroup
		for _, conn := range []*closeOnce{aCloser, bCloser} {
			closeWorkers.Add(1)
			go func(conn *closeOnce) { defer closeWorkers.Done(); _ = r.protocolCall(conn.Close) }(conn)
		}
		closeWorkers.Wait()
	}
	<-results
	workers.Wait()
}
