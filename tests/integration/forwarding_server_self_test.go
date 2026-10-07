package integration

import (
	"bytes"
	"io"
	"net"
	"sync"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

func forwardingFixtureClient(t *testing.T, server *forwardingTestServer) (*ssh.Client, forwardingTransportID) {
	t.Helper()
	raw, err := net.DialTimeout("tcp", server.address, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	_ = raw.SetDeadline(time.Now().Add(2 * time.Second))
	conn, channels, requests, err := ssh.NewClientConn(raw, server.address, &ssh.ClientConfig{
		User: "tester", Auth: []ssh.AuthMethod{ssh.Password(server.config.Password)}, HostKeyCallback: ssh.FixedHostKey(server.signer.PublicKey()),
	})
	if err != nil {
		_ = raw.Close()
		t.Fatal(err)
	}
	_ = raw.SetDeadline(time.Time{})
	client := ssh.NewClient(conn, channels, requests)
	t.Cleanup(func() { _ = client.Close() })
	return client, server.WaitTransport(t)
}

// A disposable loopback service streams bytes back and preserves half-close.
func startForwardingDestination(t *testing.T, addresses ...string) (string, func()) {
	t.Helper()
	address := "127.0.0.1:0"
	if len(addresses) != 0 {
		address = addresses[0]
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	sockets := make(map[net.Conn]struct{})
	closing := false
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		for {
			socket, err := listener.Accept()
			if err != nil {
				return
			}
			mu.Lock()
			if closing {
				mu.Unlock()
				_ = socket.Close()
				return
			}
			sockets[socket] = struct{}{}
			workers.Add(1)
			mu.Unlock()
			go func() {
				defer workers.Done()
				defer func() { _ = socket.Close(); mu.Lock(); delete(sockets, socket); mu.Unlock() }()
				_, _ = io.Copy(socket, socket)
				// This suffix can only arrive after the requester's write EOF.
				_, _ = socket.Write([]byte("after-eof"))
			}()
		}
	}()
	var once sync.Once
	closeService := func() {
		once.Do(func() {
			mu.Lock()
			closing = true
			_ = listener.Close()
			for socket := range sockets {
				_ = socket.Close()
			}
			mu.Unlock()
			workers.Wait()
		})
	}
	t.Cleanup(closeService)
	return listener.Addr().String(), closeService
}

func forwardingFixtureWait[T any](t *testing.T, result <-chan T) T {
	t.Helper()
	select {
	case value := <-result:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("forwarding fixture worker did not converge")
	}
	var zero T
	return zero
}

func TestForwardingFixtureByteIntegrityAndIndependentTransports(t *testing.T) {
	address, closeService := startForwardingDestination(t)
	server := startForwardingServer(t, forwardingServerConfig{})
	first, firstID := forwardingFixtureClient(t, server)
	second, secondID := forwardingFixtureClient(t, server)
	if secondID <= firstID {
		t.Fatalf("transport IDs = %d, %d", firstID, secondID)
	}
	connections := make([]net.Conn, 0, 4)
	for _, client := range []*ssh.Client{first, second} {
		for range 2 {
			conn, err := client.Dial("tcp", address)
			if err != nil {
				t.Fatal(err)
			}
			connections = append(connections, conn)
			t.Cleanup(func() { _ = conn.Close() })
		}
	}
	payload := bytes.Repeat([]byte{0, 255, 13, 10, 128, 42}, 32768)
	results := make(chan error, len(connections))
	for _, conn := range connections {
		go func() {
			written := make(chan error, 1)
			go func() {
				_, err := conn.Write(payload)
				if err == nil {
					err = conn.(interface{ CloseWrite() error }).CloseWrite()
				}
				written <- err
			}()
			response, err := io.ReadAll(conn)
			writeErr := <-written
			if err == nil {
				err = writeErr
			}
			if err == nil && !bytes.Equal(response, append(bytes.Clone(payload), []byte("after-eof")...)) {
				err = io.ErrUnexpectedEOF
			}
			results <- err
		}()
	}
	for range connections {
		if err := forwardingFixtureWait(t, results); err != nil {
			t.Fatal(err)
		}
	}
	// Closing an idle live channel must not affect the other transport.
	idle, err := first.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	closed := make(chan struct{})
	go func() { server.CloseTransport(firstID); close(closed) }()
	forwardingFixtureWait(t, closed)
	_ = idle.Close()
	if err := first.Wait(); err == nil {
		t.Fatal("closed transport remained connected")
	}
	survivor, err := second.Dial("tcp", address)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = survivor.Close() })
	traffic := make(chan error, 1)
	go func() {
		_, err := survivor.Write([]byte("still-forwarding"))
		if err == nil {
			err = survivor.(interface{ CloseWrite() error }).CloseWrite()
		}
		response, readErr := io.ReadAll(survivor)
		if err == nil {
			err = readErr
		}
		if err == nil && string(response) != "still-forwardingafter-eof" {
			err = io.ErrUnexpectedEOF
		}
		traffic <- err
	}()
	if err := forwardingFixtureWait(t, traffic); err != nil {
		t.Fatal(err)
	}
	_ = survivor.Close()
	session, err := second.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	output, err := session.Output("unused")
	if err == nil {
		t.Fatal("unexpected exec acceptance")
	}
	_ = session.Close()
	session, err = second.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	var shell bytes.Buffer
	session.Stdout = &shell
	if err := session.Shell(); err != nil {
		t.Fatal(err)
	}
	if err := session.Wait(); err != nil {
		t.Fatal(err)
	}
	_ = session.Close()
	if shell.String() != "server-output\n" || len(output) != 0 {
		t.Fatalf("shell=%q exec=%q", shell.String(), output)
	}
	third, thirdID := forwardingFixtureClient(t, server)
	if thirdID <= secondID {
		t.Fatalf("reused transport ID %d", thirdID)
	}
	_ = third.Close()
	server.CloseTransport(firstID)
	server.CloseTransport(9999)
	server.Close()
	server.Close()
	closeService()
	for _, released := range []string{server.address, address} {
		listener, err := net.Listen("tcp", released)
		if err != nil {
			t.Fatalf("listener %s not reusable: %v", released, err)
		}
		_ = listener.Close()
	}
}

func TestForwardingFixtureControlledReplies(t *testing.T) {
	for _, global := range []bool{false, true} {
		kind := "channel"
		if global {
			kind = "global"
		}
		for _, mode := range []string{"deny", "delay", "absent"} {
			t.Run(kind+"/"+mode, func(t *testing.T) {
				seen := make(chan struct{}, 1)
				release := make(chan struct{})
				rule := forwardingReplyRule{Seen: seen, Deny: mode == "deny", Absent: mode == "absent"}
				if mode == "delay" {
					rule.Release = release
				}
				config := forwardingServerConfig{}
				if global {
					config.GlobalRules = map[string]forwardingReplyRule{"fixture-request": rule}
				} else {
					config.ChannelRules = map[string]forwardingReplyRule{"direct-tcpip": rule}
				}
				server := startForwardingServer(t, config)
				client, id := forwardingFixtureClient(t, server)
				address, _ := startForwardingDestination(t)
				result := make(chan bool, 1)
				go func() {
					if global {
						ok, _, err := client.SendRequest("fixture-request", true, nil)
						result <- err == nil && ok
						return
					}
					conn, err := client.Dial("tcp", address)
					if conn != nil {
						_ = conn.Close()
					}
					result <- err == nil
				}()
				forwardingFixtureWait(t, seen)
				if mode != "deny" {
					select {
					case <-result:
						t.Fatal("reply arrived before release/closure")
					default:
					}
				}
				if mode == "delay" {
					close(release)
				}
				if mode == "absent" {
					closed := make(chan struct{})
					go func() { server.CloseTransport(id); close(closed) }()
					forwardingFixtureWait(t, closed)
				}
				if ok := forwardingFixtureWait(t, result); ok != (mode == "delay") {
					t.Fatalf("operation success = %v for %s", ok, mode)
				}
			})
		}
	}
}

func TestForwardingFixtureDestinationFailureAndHandshakeCleanup(t *testing.T) {
	address, closeService := startForwardingDestination(t)
	closeService()
	server := startForwardingServer(t, forwardingServerConfig{})
	client, _ := forwardingFixtureClient(t, server)
	if conn, err := client.Dial("tcp", address); err == nil {
		_ = conn.Close()
		t.Fatal("unavailable destination accepted")
	}
	if session, err := client.NewSession(); err != nil {
		t.Fatal(err)
	} else {
		_ = session.Close()
	}
	raw, err := net.DialTimeout("tcp", server.address, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	closed := make(chan struct{})
	go func() { server.Close(); close(closed) }()
	forwardingFixtureWait(t, closed)
}

func TestForwardingFixtureReusableSessionHandler(t *testing.T) {
	server := startForwardingServer(t, forwardingServerConfig{Session: forwardingSessionHandler(23)})
	client, _ := forwardingFixtureClient(t, server)
	session, err := client.NewSession()
	if err != nil {
		t.Fatal(err)
	}
	defer session.Close()
	session.Stdin = bytes.NewBufferString("payload\n")
	var stdout, stderr bytes.Buffer
	session.Stdout, session.Stderr = &stdout, &stderr
	result := make(chan error, 1)
	go func() { result <- session.Run(`printf 'out\n'; printf 'err\n' >&2; cat; exit 23`) }()
	err = forwardingFixtureWait(t, result)
	status, ok := err.(*ssh.ExitError)
	if !ok || status.ExitStatus() != 23 {
		t.Fatalf("exec result = %v", err)
	}
	if stdout.String() != "out\npayload\n" || stderr.String() != "err\n" {
		t.Fatalf("stdout=%q stderr=%q", stdout.String(), stderr.String())
	}
}

func TestForwardingFixtureVendorRemoteListener(t *testing.T) {
	server := startForwardingServer(t, forwardingServerConfig{})
	client, id := forwardingFixtureClient(t, server)
	listener, err := client.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan error, 1)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			_, err = io.Copy(conn, conn)
			_ = conn.Close()
		}
		accepted <- err
	}()
	conn, err := net.DialTimeout("tcp", listener.Addr().String(), time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(time.Second))
	if _, err := conn.Write([]byte{0, 255, 42}); err != nil {
		t.Fatal(err)
	}
	var reply [3]byte
	if _, err := io.ReadFull(conn, reply[:]); err != nil || reply != [3]byte{0, 255, 42} {
		t.Fatalf("remote bytes = %v, %v", reply, err)
	}
	_ = conn.(*net.TCPConn).CloseWrite()
	if err := forwardingFixtureWait(t, accepted); err != nil {
		t.Fatal(err)
	}
	endpoint := forwardingEndpoint(t, listener.Addr().String())
	if err := listener.Close(); err != nil {
		t.Fatal(err)
	}
	forwardingPortReleased(t, endpoint)
	server.CloseTransport(id)
}
