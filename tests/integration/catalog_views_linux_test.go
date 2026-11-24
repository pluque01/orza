package integration

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/cli"
	"github.com/pluque01/orza/internal/hostkey"
	"github.com/pluque01/orza/internal/sshclient"
	"github.com/pluque01/orza/internal/terminal"
	"github.com/pluque01/orza/internal/tui"
	"golang.org/x/crypto/ssh"
)

const catalogHistoryRemoteExit = 23

func TestRecentCatalogAlternatesPTYAndCLISessions(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Second)
	defer cancel()
	server := startForwardingServer(t, forwardingServerConfig{Session: forwardingSessionHandler(catalogHistoryRemoteExit)})
	rejected := startForwardingServer(t, forwardingServerConfig{Session: func(_ context.Context, _ ssh.Channel, requests <-chan *ssh.Request) {
		for request := range requests {
			_ = request.Reply(request.Type == "pty-req", nil)
			if request.Type == "shell" {
				return
			}
		}
	}})
	fixture := newForwardingAppFixture(t, "")
	const host, rejectHost = "catalog-history.invalid", "catalog-rejected.invalid"
	addrs := map[string]string{host: server.address, rejectHost: rejected.address}
	dialer := &net.Dialer{}
	dial := func(ctx context.Context, network, address string) (net.Conn, error) {
		name, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, err
		}
		local, ok := addrs[name]
		if !ok || !strings.HasSuffix(local, ":"+port) {
			return nil, fmt.Errorf("unexpected SSH test destination %q", address)
		}
		return dialer.DialContext(ctx, network, local)
	}
	openers := make([]app.Connection, 0, 11)
	for index := range 11 {
		name := fmt.Sprintf("session-%02d", index)
		openers = append(openers, createCatalogSessionFixture(t, fixture, server, host, name))
	}
	canceled := createCatalogSessionFixture(t, fixture, server, host, "session-11-canceled")
	failed := createCatalogSessionFixture(t, fixture, rejected, rejectHost, "session-12-failed")
	commandOnly := createCatalogSessionFixture(t, fixture, server, host, "session-13-exec")
	tunnelOnly := createCatalogSessionFixture(t, fixture, server, host, "session-14-tunnel")
	model := tui.New(tui.Config{Connections: fixture.connections, Folders: fixture.folders, Width: 100, Height: 24, NoColor: true})
	executeTUICommand(t, model, model.Init())

	for index, connection := range openers {
		t.Run(connection.Name, func(t *testing.T) {
			if index%2 == 0 {
				openCatalogSessionInTUI(t, ctx, fixture, dial, index, connection.Name, false)
			} else if err := runCatalogConnectCLI(t, ctx, fixture, dial, connection.Path); cli.ExitCode(err) != catalogHistoryRemoteExit {
				t.Fatalf("connect exit = %d, error = %v", cli.ExitCode(err), err)
			}
			got := getCatalogSession(t, ctx, fixture, connection.ID)
			if got.LastOpenedAt.IsZero() {
				t.Fatalf("%s did not record a shell with remote exit status %d", connection.Path, catalogHistoryRemoteExit)
			}
		})
	}
	// This browser was loaded before any SSH session opened. Entering Recent
	// must refresh the projection and report openings by both entry points.
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("[")))
	want := make([]string, 0, 10)
	for i := 10; i >= 1; i-- {
		want = append(want, openers[i].Path)
	}
	assertRecentCatalogPaths(t, model, want)

	reopen := openers[5]
	if err := runCatalogConnectCLI(t, ctx, fixture, dial, reopen.Path); cli.ExitCode(err) != catalogHistoryRemoteExit {
		t.Fatalf("reopen exit = %d, error = %v", cli.ExitCode(err), err)
	}
	updateTUI(t, model, tea.KeyPressMsg(tea.Key{Code: tea.KeyF3}))
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("[")))
	want = append([]string{reopen.Path}, slices.Delete(want, 5, 6)...)
	assertRecentCatalogPaths(t, model, want)

	openCatalogSessionInTUI(t, ctx, fixture, dial, 11, canceled.Name, true)
	if err := runCatalogConnectCLI(t, ctx, fixture, dial, failed.Path); cli.ExitCode(err) != cli.ExitTransport {
		t.Fatalf("rejected shell exit = %d, error = %v", cli.ExitCode(err), err)
	}
	runner := sshclient.New(sshclient.Options{DialContext: dial, Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	options := fixture.options
	commands, err := app.NewCommandService(app.CommandOptions{Connections: options.Connections, Credentials: options.Credentials, HostTrust: options.HostTrust, TrustedHosts: options.TrustedHosts, Store: options.Store, Scope: options.Scope, Runner: runner})
	if err != nil {
		t.Fatal(err)
	}
	root := cli.NewRoot(cli.RootConfig{Command: commands, Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	root.SetArgs([]string{"exec", commandOnly.Path, "--", `printf 'out\n'; printf 'err\n' >&2; cat; exit 23`})
	if err := root.ExecuteContext(ctx); cli.ExitCode(err) != catalogHistoryRemoteExit {
		t.Fatalf("exec exit = %d, error = %v", cli.ExitCode(err), err)
	}
	options.Runner = runner
	tunnels, err := app.NewTunnelService(options)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = tunnels.Close() })
	tunnel, err := tunnels.Start(ctx, forwardingAppRequest(tunnelOnly, app.TunnelConfig{
		Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: app.TunnelEndpoint{Host: "catalog-destination.invalid", Port: 1234},
	}))
	if err != nil {
		t.Fatal(err)
	}
	forwardingAppState(t, tunnels, tunnel.ID, app.TunnelActive)
	if err := tunnels.Stop(ctx, tunnel.ID); err != nil {
		t.Fatal(err)
	}
	for _, connection := range []app.Connection{canceled, failed, commandOnly, tunnelOnly} {
		if got := getCatalogSession(t, ctx, fixture, connection.ID); !got.LastOpenedAt.IsZero() {
			t.Errorf("%s unexpectedly became Recent: %v", connection.Path, got.LastOpenedAt)
		}
	}
	executeTUICommand(t, model, updateTUI(t, model, tuiKey("r")))
	assertRecentCatalogPaths(t, model, want)
}

func createCatalogSessionFixture(t *testing.T, fixture *forwardingAppFixture, server *forwardingTestServer, host, name string) app.Connection {
	t.Helper()
	ctx := t.Context()
	port := forwardingEndpoint(t, server.address).Port
	presented, err := hostkey.NewPresentedHost(app.HostEndpoint{CanonicalHost: host, Port: port}, server.address, server.signer.PublicKey())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.trusted.TrustHost(ctx, app.TrustHostRequest{Host: presented}); err != nil {
		t.Fatal(err)
	}
	result, err := fixture.connections.Create(ctx, app.CreateConnectionRequest{
		Parent: app.ItemSelector{Path: "/"}, Name: name, Host: host, Port: port, Username: "tester", AuthMethod: app.AuthMethodPassword,
		CredentialIntent: app.CredentialRemember, Password: []byte(server.config.Password),
	})
	if err != nil {
		t.Fatal(err)
	}
	return result.Connection
}

func getCatalogSession(t *testing.T, ctx context.Context, fixture *forwardingAppFixture, id app.NodeID) app.Connection {
	t.Helper()
	result, err := fixture.connections.Get(ctx, app.ItemSelector{ID: id})
	if err != nil {
		t.Fatal(err)
	}
	return result.Connection
}

func catalogConnectService(t *testing.T, fixture *forwardingAppFixture, local app.Terminal, runner *sshclient.Client) *app.ConnectService {
	t.Helper()
	options := fixture.options
	service, err := app.NewConnectService(app.ConnectOptions{
		Connections: options.Connections, Credentials: options.Credentials, HostTrust: options.HostTrust,
		TrustedHosts: options.TrustedHosts, Store: options.Store, Scope: options.Scope, Terminal: local, Runner: runner,
	})
	if err != nil {
		t.Fatal(err)
	}
	return service
}

func runCatalogConnectCLI(t *testing.T, ctx context.Context, fixture *forwardingAppFixture, dial func(context.Context, string, string) (net.Conn, error), path string) error {
	t.Helper()
	_, input := openShellTestPTY(t)
	local := terminal.New(input, input)
	runner := sshclient.New(sshclient.Options{DialContext: dial, Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})
	service := catalogConnectService(t, fixture, local, runner)
	root := cli.NewRoot(cli.RootConfig{Connections: fixture.connections, Connect: service, Terminal: local, Stdin: input, Stdout: io.Discard, Stderr: io.Discard})
	root.SetArgs([]string{"connect", path})
	return root.ExecuteContext(ctx)
}

func openCatalogSessionInTUI(t *testing.T, ctx context.Context, fixture *forwardingAppFixture, dial func(context.Context, string, string) (net.Conn, error), index int, name string, cancelConnect bool) {
	t.Helper()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	keyboard, input := openShellTestPTY(t)
	local := terminal.New(input, input)
	const ready, confirm, returned = "[ssh] session-01", "Connect to SSH target?", "Shell closed with status 23."
	selection := "/" + name
	favourite := "* " + name
	frames := newSignalingWriter(ready, selection, confirm, returned, favourite, "[Favourites]")
	runner := sshclient.New(sshclient.Options{DialContext: dial, Stdin: input, Stdout: io.Discard, Stderr: io.Discard})
	service := catalogConnectService(t, fixture, local, runner)
	model := tui.New(tui.Config{Connections: fixture.connections, Folders: fixture.folders, Connect: service, Terminal: local, Stdin: input, NoColor: true, Context: ctx})
	program := tea.NewProgram(catalogPTYModel{Model: model, frames: frames}, tea.WithInput(input), tea.WithOutput(io.Discard), tea.WithWindowSize(80, 24), tea.WithoutSignalHandler())
	done := make(chan struct{})
	var runErr error
	go func() {
		defer close(done)
		_, runErr = program.Run()
	}()
	defer func() {
		// Cancel app operations first, then let Bubble Tea join its input
		// reader and restore the terminal before the PTY files are closed.
		cancel()
		program.Send(tea.Quit())
		<-done
	}()
	step := func(frame, keys string) {
		t.Helper()
		select {
		case <-frames.signal(frame):
		case <-done:
			t.Fatalf("TUI stopped before %q: %v", frame, runErr)
		case <-ctx.Done():
			t.Fatalf("waiting for %q: %s", frame, frames.String())
		}
		if _, err := io.WriteString(keyboard, keys); err != nil {
			t.Fatal(err)
		}
	}
	step(ready, strings.Repeat("j", index+1))
	if index == 0 {
		step(selection, "\x1bOS") // xterm F4
		step(favourite, "\x1bOR") // xterm F3
		step("[Favourites]", "c")
	} else {
		step(selection, "c")
	}
	if cancelConnect {
		step(confirm, "\rq")
	} else {
		step(confirm, "y")
		step(returned, "q")
	}
	select {
	case <-done:
		if runErr != nil {
			t.Fatal(runErr)
		}
	case <-ctx.Done():
		t.Fatalf("TUI did not quit on the PTY: %s", frames.String())
	}
}

// Observe complete frames independently of the renderer's terminal diffs.
// Keyboard input still reaches the model through the real PTY and Bubble Tea.
type catalogPTYModel struct {
	*tui.Model
	frames *signalingWriter
}

func (m catalogPTYModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, command := m.Model.Update(msg)
	return m, command
}

func (m catalogPTYModel) View() tea.View {
	view := m.Model.View()
	_, _ = io.WriteString(m.frames, view.Content)
	return view
}

func assertRecentCatalogPaths(t *testing.T, model *tui.Model, want []string) {
	t.Helper()
	var paths []string
	view := model.View().Content
	for _, line := range strings.Split(view, "\n") {
		_, rest, ok := strings.Cut(line, "│")
		if !ok {
			continue
		}
		cell, _, _ := strings.Cut(rest, "│")
		if path := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(cell), ">")); strings.HasPrefix(path, "/session-") {
			paths = append(paths, path)
		}
	}
	if !slices.Equal(paths, want) {
		t.Fatalf("Recent paths = %q, want %q:\n%s", paths, want, view)
	}
}
