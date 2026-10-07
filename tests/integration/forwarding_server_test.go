package integration

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"errors"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// Rules are immutable after startup. Release gates give tests deterministic
// delayed replies; Absent holds the operation until its transport is closed.
type forwardingReplyRule struct {
	Deny    bool
	Delay   time.Duration
	Release <-chan struct{}
	Absent  bool
	Seen    chan<- struct{}
}

type forwardingServerConfig struct {
	Password     string
	ChannelRules map[string]forwardingReplyRule
	GlobalRules  map[string]forwardingReplyRule
	Session      func(context.Context, ssh.Channel, <-chan *ssh.Request)
	// A loopback override simulates server policy differing from requested scope.
	RemoteBindHost string
	RejectMessage  string
	// Test-only host-side DNS aliases; keys are received SSH destination names.
	DestinationAliases map[string]string
}

type forwardingTransportID uint64

type forwardingTransport struct {
	ctx              context.Context
	cancel           context.CancelFunc
	raw              net.Conn
	done             chan struct{}
	mu               sync.Mutex
	sockets          map[net.Conn]struct{}
	listeners        map[string]net.Listener
	conn             *ssh.ServerConn
	workers          sync.WaitGroup
	readHold         chan struct{}
	readBlocked      chan struct{}
	readOnce         sync.Once
	channels         map[string]uint64
	acceptedChannels map[string]uint64
}

type forwardingControlledConn struct {
	net.Conn
	transport *forwardingTransport
}

func (c *forwardingControlledConn) Read(p []byte) (int, error) {
	n, err := c.Conn.Read(p)
	c.transport.mu.Lock()
	hold := c.transport.readHold
	c.transport.mu.Unlock()
	if hold != nil {
		c.transport.readOnce.Do(func() { close(c.transport.readBlocked) })
		select {
		case <-hold:
		case <-c.transport.ctx.Done():
			return 0, c.transport.ctx.Err()
		}
	}
	return n, err
}

type forwardingTestServer struct {
	address    string
	signer     ssh.Signer
	config     forwardingServerConfig
	listener   net.Listener
	ctx        context.Context
	cancel     context.CancelFunc
	mu         sync.Mutex
	nextID     forwardingTransportID
	transports map[forwardingTransportID]*forwardingTransport
	accepted   chan forwardingTransportID
	wait       sync.WaitGroup
	closeOnce  sync.Once
}

func startForwardingServer(t *testing.T, config forwardingServerConfig) *forwardingTestServer {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	if config.Password == "" {
		config.Password = "test-password"
	}
	if config.Session == nil {
		config.Session = forwardingSessionHandler(0)
	}
	if config.RemoteBindHost != "" && !net.ParseIP(config.RemoteBindHost).IsLoopback() {
		t.Fatal("fixture remote bind override must be loopback")
	}
	ctx, cancel := context.WithCancel(context.Background())
	s := &forwardingTestServer{address: listener.Addr().String(), signer: signer, config: config, listener: listener, ctx: ctx, cancel: cancel, transports: make(map[forwardingTransportID]*forwardingTransport), accepted: make(chan forwardingTransportID, 64)}
	sshConfig := &ssh.ServerConfig{PasswordCallback: func(metadata ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
		if metadata.User() != "tester" || string(password) != config.Password {
			return nil, errors.New("authentication rejected")
		}
		return nil, nil
	}}
	sshConfig.AddHostKey(signer)
	s.wait.Add(1)
	go func() {
		defer s.wait.Done()
		for {
			raw, err := listener.Accept()
			if err != nil {
				return
			}
			ctx, cancel := context.WithCancel(s.ctx)
			tr := &forwardingTransport{ctx: ctx, cancel: cancel, done: make(chan struct{}), sockets: make(map[net.Conn]struct{}), listeners: make(map[string]net.Listener), channels: make(map[string]uint64), acceptedChannels: make(map[string]uint64), readBlocked: make(chan struct{})}
			tr.raw = &forwardingControlledConn{Conn: raw, transport: tr}
			s.mu.Lock()
			if s.ctx.Err() != nil {
				s.mu.Unlock()
				cancel()
				_ = raw.Close()
				return
			}
			s.nextID++
			id := s.nextID
			s.transports[id] = tr
			s.wait.Add(1)
			s.mu.Unlock()
			go s.serve(id, tr, sshConfig)
		}
	}()
	t.Cleanup(s.Close)
	return s
}

func (s *forwardingTestServer) serve(id forwardingTransportID, tr *forwardingTransport, config *ssh.ServerConfig) {
	defer s.wait.Done()
	defer close(tr.done)
	defer tr.close()
	conn, channels, requests, err := ssh.NewServerConn(tr.raw, config)
	if err != nil {
		return
	}
	tr.mu.Lock()
	tr.conn = conn
	tr.mu.Unlock()
	select {
	case s.accepted <- id:
	case <-tr.ctx.Done():
		return
	}
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		for request := range requests {
			rule, configured := s.config.GlobalRules[request.Type]
			if !rule.await(tr.ctx) {
				return
			}
			if rule.Deny {
				_ = request.Reply(false, nil)
				continue
			}
			switch request.Type {
			case "tcpip-forward", "cancel-tcpip-forward":
				s.handleRemoteRequest(tr, request)
			default:
				_ = request.Reply(configured, nil)
			}
		}
	}()
	for incoming := range channels {
		workers.Add(1)
		go func(incoming ssh.NewChannel) {
			defer workers.Done()
			s.handleChannel(tr, incoming)
		}(incoming)
	}
	tr.close()
	_ = conn.Close()
	workers.Wait()
	tr.workers.Wait()
}

// Worker admission and resource registration share the cancellation lock, so
// cleanup cannot miss late sockets, listeners, or worker additions.
func (tr *forwardingTransport) startWorker(work func()) bool {
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.ctx.Err() != nil {
		return false
	}
	tr.workers.Add(1)
	go func() { defer tr.workers.Done(); work() }()
	return true
}

func (s *forwardingTestServer) handleRemoteRequest(tr *forwardingTransport, request *ssh.Request) {
	var payload struct {
		Host string
		Port uint32
	}
	if ssh.Unmarshal(request.Payload, &payload) != nil || payload.Port > 65535 || !net.ParseIP(payload.Host).IsLoopback() {
		_ = request.Reply(false, nil)
		return
	}
	key := net.JoinHostPort(payload.Host, strconv.Itoa(int(payload.Port)))
	if request.Type == "cancel-tcpip-forward" {
		tr.mu.Lock()
		listener := tr.listeners[key]
		delete(tr.listeners, key)
		tr.mu.Unlock()
		if listener != nil {
			_ = listener.Close()
		}
		_ = request.Reply(listener != nil, nil)
		return
	}
	bindHost := payload.Host
	if s.config.RemoteBindHost != "" {
		bindHost = s.config.RemoteBindHost
	}
	listener, err := net.Listen("tcp", net.JoinHostPort(bindHost, strconv.Itoa(int(payload.Port))))
	if err != nil {
		_ = request.Reply(false, nil)
		return
	}
	port := uint32(listener.Addr().(*net.TCPAddr).Port)
	key = net.JoinHostPort(payload.Host, strconv.Itoa(int(port)))
	tr.mu.Lock()
	if tr.ctx.Err() != nil {
		tr.mu.Unlock()
		_ = listener.Close()
		return
	}
	tr.listeners[key] = listener
	tr.mu.Unlock()
	var reply []byte
	if payload.Port == 0 {
		reply = ssh.Marshal(struct{ Port uint32 }{port})
	}
	if err := request.Reply(true, reply); err != nil {
		_ = listener.Close()
		return
	}
	tr.startWorker(func() {
		for {
			socket, err := listener.Accept()
			if err != nil {
				return
			}
			tr.mu.Lock()
			if tr.ctx.Err() != nil {
				tr.mu.Unlock()
				_ = socket.Close()
				return
			}
			tr.sockets[socket] = struct{}{}
			tr.mu.Unlock()
			if !tr.startWorker(func() {
				defer func() { _ = socket.Close(); tr.mu.Lock(); delete(tr.sockets, socket); tr.mu.Unlock() }()
				origin := socket.RemoteAddr().(*net.TCPAddr)
				channel, requests, err := s.openForwarded(tr, payload.Host, port, origin.IP.String(), uint32(origin.Port))
				if err != nil {
					return
				}
				forwardingBridge(channel, requests, socket)
			}) {
				_ = socket.Close()
				return
			}
		}
	})
}

func (s *forwardingTestServer) openForwarded(tr *forwardingTransport, host string, port uint32, origin string, originPort uint32) (ssh.Channel, <-chan *ssh.Request, error) {
	rule := s.config.ChannelRules["forwarded-tcpip"]
	if !rule.await(tr.ctx) {
		return nil, nil, tr.ctx.Err()
	}
	if rule.Deny {
		return nil, nil, errors.New("forwarded channel denied")
	}
	tr.mu.Lock()
	tr.channels["forwarded-tcpip"]++
	conn := tr.conn
	tr.mu.Unlock()
	channel, requests, err := conn.OpenChannel("forwarded-tcpip", ssh.Marshal(struct {
		Host       string
		Port       uint32
		Origin     string
		OriginPort uint32
	}{host, port, origin, originPort}))
	if err == nil {
		tr.mu.Lock()
		tr.acceptedChannels["forwarded-tcpip"]++
		tr.mu.Unlock()
	}
	return channel, requests, err
}

func (r forwardingReplyRule) await(ctx context.Context) bool {
	if r.Seen != nil {
		select {
		case r.Seen <- struct{}{}:
		default:
		}
	}
	if r.Absent {
		<-ctx.Done()
		return false
	}
	if r.Release != nil {
		select {
		case <-r.Release:
		case <-ctx.Done():
			return false
		}
	}
	if r.Delay > 0 {
		timer := time.NewTimer(r.Delay)
		defer timer.Stop()
		select {
		case <-timer.C:
		case <-ctx.Done():
			return false
		}
	}
	return ctx.Err() == nil
}

func (s *forwardingTestServer) handleChannel(tr *forwardingTransport, incoming ssh.NewChannel) {
	tr.mu.Lock()
	tr.channels[incoming.ChannelType()]++
	tr.mu.Unlock()
	rule := s.config.ChannelRules[incoming.ChannelType()]
	if !rule.await(tr.ctx) {
		return
	}
	if rule.Deny {
		message := s.config.RejectMessage
		if message == "" {
			message = "forwarding denied"
		}
		_ = incoming.Reject(ssh.Prohibited, message)
		return
	}
	switch incoming.ChannelType() {
	case "session":
		channel, requests, err := incoming.Accept()
		if err != nil {
			return
		}
		defer channel.Close()
		tr.mu.Lock()
		tr.acceptedChannels["session"]++
		tr.mu.Unlock()
		s.config.Session(tr.ctx, channel, requests)
	case "direct-tcpip":
		var request struct {
			Host       string
			Port       uint32
			OriginHost string
			OriginPort uint32
		}
		if ssh.Unmarshal(incoming.ExtraData(), &request) != nil || request.Port == 0 || request.Port > 65535 {
			_ = incoming.Reject(ssh.ConnectionFailed, "invalid destination")
			return
		}
		host := request.Host
		if alias := s.config.DestinationAliases[host]; alias != "" {
			host = alias
		}
		socket, err := (&net.Dialer{}).DialContext(tr.ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(int(request.Port))))
		if err != nil {
			_ = incoming.Reject(ssh.ConnectionFailed, "destination unavailable")
			return
		}
		tr.mu.Lock()
		if tr.ctx.Err() != nil {
			tr.mu.Unlock()
			_ = socket.Close()
			return
		}
		tr.sockets[socket] = struct{}{}
		tr.mu.Unlock()
		defer func() { _ = socket.Close(); tr.mu.Lock(); delete(tr.sockets, socket); tr.mu.Unlock() }()
		channel, requests, err := incoming.Accept()
		if err != nil {
			return
		}
		tr.mu.Lock()
		tr.acceptedChannels["direct-tcpip"]++
		tr.mu.Unlock()
		forwardingBridge(channel, requests, socket)
	default:
		_ = incoming.Reject(ssh.UnknownChannelType, "unsupported channel")
	}
}

func forwardingBridge(channel ssh.Channel, requests <-chan *ssh.Request, socket net.Conn) {
	defer channel.Close()
	drained := make(chan struct{})
	go func() { ssh.DiscardRequests(requests); close(drained) }()
	copied := make(chan struct{})
	go func() {
		_, err := io.Copy(socket, channel)
		if err != nil {
			_ = socket.Close()
			_ = channel.Close()
		} else if tcp, ok := socket.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
		close(copied)
	}()
	_, err := io.Copy(channel, socket)
	if err != nil {
		_ = socket.Close()
		_ = channel.Close()
	} else {
		_ = channel.CloseWrite()
	}
	<-copied
	_ = channel.Close()
	<-drained
}

// Mirrors the existing fixture's shell/exec protocol without coupling its
// single-transport lifecycle to forwarding tests. Custom handlers may wait on ctx.
func forwardingSessionHandler(status uint32) func(context.Context, ssh.Channel, <-chan *ssh.Request) {
	return func(_ context.Context, channel ssh.Channel, requests <-chan *ssh.Request) {
		for request := range requests {
			switch request.Type {
			case "pty-req":
				_ = request.Reply(true, nil)
			case "shell":
				_ = request.Reply(true, nil)
				_, _ = io.WriteString(channel, "server-output\n")
			case "exec":
				var payload struct{ Command string }
				if ssh.Unmarshal(request.Payload, &payload) != nil || payload.Command != `printf 'out\n'; printf 'err\n' >&2; cat; exit 23` {
					_ = request.Reply(false, nil)
					continue
				}
				_ = request.Reply(true, nil)
				_, _ = io.WriteString(channel, "out\n")
				_, _ = io.WriteString(channel.Stderr(), "err\n")
				_, _ = io.Copy(channel, channel)
			default:
				_ = request.Reply(false, nil)
				continue
			}
			if request.Type == "pty-req" {
				continue
			}
			_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
			return
		}
	}
}

func (tr *forwardingTransport) close() {
	tr.cancel()
	_ = tr.raw.Close()
	tr.mu.Lock()
	defer tr.mu.Unlock()
	for socket := range tr.sockets {
		_ = socket.Close()
	}
	for _, listener := range tr.listeners {
		_ = listener.Close()
	}
}

func (s *forwardingTestServer) transport(id forwardingTransportID) *forwardingTransport {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.transports[id]
}

func (s *forwardingTestServer) ChannelCount(id forwardingTransportID, kind string) uint64 {
	tr := s.transport(id)
	if tr == nil {
		return 0
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.channels[kind]
}

func (s *forwardingTestServer) AcceptedChannelCount(id forwardingTransportID, kind string) uint64 {
	tr := s.transport(id)
	if tr == nil {
		return 0
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	return tr.acceptedChannels[kind]
}

func (s *forwardingTestServer) RemoteAddress(id forwardingTransportID, requested string) string {
	tr := s.transport(id)
	if tr == nil {
		return ""
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if listener := tr.listeners[requested]; listener != nil {
		return listener.Addr().String()
	}
	return ""
}

// HoldTransportReads stalls the real vendor packet reader, including automatic
// channel-close replies. It is released by transport closure, not a fake mux.
func (s *forwardingTestServer) HoldTransportReads(id forwardingTransportID) <-chan struct{} {
	tr := s.transport(id)
	if tr == nil {
		return nil
	}
	tr.mu.Lock()
	defer tr.mu.Unlock()
	if tr.readHold == nil {
		tr.readHold = make(chan struct{})
	}
	return tr.readBlocked
}

// Flood opens a finite number of real forwarded channels concurrently, without
// destination sockets. All open/read workers remain owned by this transport.
func (s *forwardingTestServer) Flood(id forwardingTransportID, host string, port uint16, count int) <-chan struct{} {
	done := make(chan struct{})
	tr := s.transport(id)
	if tr == nil {
		close(done)
		return done
	}
	if !tr.startWorker(func() {
		defer close(done)
		var wait sync.WaitGroup
		for range count {
			wait.Add(1)
			if !tr.startWorker(func() {
				defer wait.Done()
				channel, requests, err := s.openForwarded(tr, host, uint32(port), "127.0.0.1", 12345)
				if err != nil {
					return
				}
				defer channel.Close()
				drained := make(chan struct{})
				go func() { ssh.DiscardRequests(requests); close(drained) }()
				_, _ = io.Copy(io.Discard, channel)
				_ = channel.Close()
				<-drained
			}) {
				wait.Done()
			}
		}
		wait.Wait()
	}) {
		close(done)
	}
	return done
}

// IDs are assigned in TCP accept order, never reused, and announced after auth.
func (s *forwardingTestServer) WaitTransport(t *testing.T) forwardingTransportID {
	t.Helper()
	select {
	case id := <-s.accepted:
		return id
	case <-time.After(2 * time.Second):
		t.Fatal("forwarding transport did not authenticate")
	}
	return 0
}

// CloseTransport forces raw closure and joins all that transport's workers.
func (s *forwardingTestServer) CloseTransport(id forwardingTransportID) {
	s.mu.Lock()
	tr := s.transports[id]
	s.mu.Unlock()
	if tr == nil {
		return
	}
	tr.close()
	<-tr.done
}

func (s *forwardingTestServer) Close() {
	s.closeOnce.Do(func() {
		s.cancel()
		_ = s.listener.Close()
		s.mu.Lock()
		for _, tr := range s.transports {
			tr.close()
		}
		s.mu.Unlock()
		s.wait.Wait()
	})
}
