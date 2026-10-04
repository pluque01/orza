package integration

import (
	"bytes"
	"context"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/sshclient"
	"github.com/pluque01/orza/internal/terminal"
	"github.com/pluque01/orza/internal/tui"
	"golang.org/x/crypto/ssh"
)

func TestTunnelFailureDuringShellReconcilesOnReturn(t *testing.T) {
	for _, outcome := range []string{"normal", "nonzero", "shell-transport-failure"} {
		t.Run(outcome, func(t *testing.T) {
			shellStarted, releaseShell := make(chan struct{}), make(chan struct{})
			status := uint32(0)
			if outcome == "nonzero" {
				status = 23
			}
			server := startForwardingServer(t, forwardingServerConfig{Session: func(ctx context.Context, channel ssh.Channel, requests <-chan *ssh.Request) {
				for request := range requests {
					switch request.Type {
					case "pty-req":
						_ = request.Reply(true, nil)
					case "shell":
						_ = request.Reply(true, nil)
						close(shellStarted)
						select {
						case <-releaseShell:
						case <-ctx.Done():
							return
						}
						_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{status}))
						return
					default:
						_ = request.Reply(false, nil)
					}
				}
			}})
			fixture := newForwardingAppFixture(t, "")
			connection := fixture.create(t, server, "shell-host")
			fixture.trust(t, server)
			address, _ := startForwardingDestination(t)
			firstConfig := app.TunnelConfig{Mode: app.TunnelLocal, Listen: forwardingFreeEndpoint(t), Destination: forwardingEndpoint(t, address)}
			secondConfig := firstConfig
			secondConfig.Listen = forwardingFreeEndpoint(t)
			first, err := fixture.service.Start(context.Background(), forwardingAppRequest(connection, firstConfig))
			if err != nil {
				t.Fatal(err)
			}
			forwardingAppState(t, fixture.service, first.ID, app.TunnelActive)
			firstTransport := server.WaitTransport(t)
			second, err := fixture.service.Start(context.Background(), forwardingAppRequest(connection, secondConfig))
			if err != nil {
				t.Fatal(err)
			}
			forwardingAppState(t, fixture.service, second.ID, app.TunnelActive)
			_ = server.WaitTransport(t)
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			options := fixture.options
			connect, err := app.NewConnectService(app.ConnectOptions{Connections: options.Connections, Credentials: options.Credentials, HostTrust: options.HostTrust, TrustedHosts: options.TrustedHosts, Store: options.Store, Scope: options.Scope, Terminal: local, Runner: sshclient.New(sshclient.Options{Stdin: bytes.NewReader(nil), Stdout: io.Discard, Stderr: io.Discard})})
			if err != nil {
				t.Fatal(err)
			}
			returned := "Shell closed"
			if outcome == "shell-transport-failure" {
				returned = "Shell connection ended; browser restored."
			}
			writer := newSignalingWriter("[ssh] shell-host", "Endpoint", "Connect to SSH target?", returned, "Quit and close all live tunnels?")
			input := pipeInput(t, []readerStage{{wait: writer.signal("[ssh] shell-host"), data: "l"}, {wait: writer.signal("Endpoint"), data: "c"}, {wait: writer.signal("Connect to SSH target?"), data: "y"}, {wait: writer.signal(returned), data: "q"}, {wait: writer.signal("Quit and close all live tunnels?"), data: "y"}})
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			type runResult struct {
				result tui.Result
				err    error
			}
			done := make(chan runResult, 1)
			joined := make(chan struct{})
			go func() {
				defer close(joined)
				result, err := tui.Run(ctx, tui.Config{Connections: fixture.connections, Folders: fixture.folders, Connect: connect, Tunnels: fixture.service, Terminal: local, Stdin: input, Stdout: writer, NoColor: true})
				done <- runResult{result, err}
			}()
			t.Cleanup(func() {
				cancel()
				select {
				case <-joined:
				case <-time.After(3 * time.Second):
					t.Error("shell TUI worker did not join")
				}
			})
			select {
			case <-shellStarted:
			case <-ctx.Done():
				t.Fatal("shell never acquired terminal")
			}
			shellTransport := server.WaitTransport(t)
			for _, config := range []app.TunnelConfig{firstConfig, secondConfig} {
				if err := forwardingExchange(forwardingDial(t, config, config.Destination), []byte("during-shell")); err != nil {
					t.Fatal(err)
				}
			}
			server.CloseTransport(firstTransport)
			forwardingAppState(t, fixture.service, first.ID, app.TunnelFailed)
			forwardingPortReleased(t, firstConfig.Listen)
			if err := forwardingExchange(forwardingDial(t, secondConfig, secondConfig.Destination), []byte("unrelated-tunnel-continuity")); err != nil {
				t.Fatal(err)
			}
			if outcome == "shell-transport-failure" {
				server.CloseTransport(shellTransport)
			} else {
				close(releaseShell)
			}
			select {
			case result := <-done:
				if result.err != nil || result.result.RemoteExitStatus != nil || !result.result.Session.StartedAt.IsZero() {
					t.Fatalf("TUI retained shell result: %+v %v", result.result, result.err)
				}
			case <-ctx.Done():
				t.Fatalf("browser did not safely quit after shell return: %s", writer.String())
			}
			frame := writer.String()
			for _, text := range []string{returned, "Failed", "Tunnels (1 active)", "Quit and close all live tunnels?"} {
				if !strings.Contains(frame, text) {
					t.Fatalf("shell return omitted %q", text)
				}
			}
			forwardingPortReleased(t, secondConfig.Listen)
			assertTerminalRestored(t, local)
		})
	}
}

func TestTunnelFormStartsAndStopsAllModesThroughTUI(t *testing.T) {
	for _, mode := range []app.TunnelMode{app.TunnelLocal, app.TunnelRemote, app.TunnelDynamic} {
		for _, size := range []terminal.Size{{Columns: 80, Rows: 24}, {Columns: 40, Rows: 12}} {
			t.Run(string(mode)+"/"+strconv.Itoa(size.Columns), func(t *testing.T) {
				server := startForwardingServer(t, forwardingServerConfig{})
				fixture := newForwardingAppFixture(t, "")
				connection := fixture.create(t, server, "ui-host")
				fixture.trust(t, server)
				address, _ := startForwardingDestination(t)
				destination := forwardingEndpoint(t, address)
				listen := forwardingFreeEndpoint(t)
				writer := newSignalingWriter("q Quit  |  ? Help", "Connection", "Mode: local", "Start forwarding?", "Tunnels (1 active)", "s Stop", "Stop this tunnel?", "Stopped")
				modeKeys := ""
				if mode == app.TunnelRemote {
					modeKeys = "\x1b[C"
				} else if mode == app.TunnelDynamic {
					modeKeys = "\x1b[C\x1b[C"
				}
				fields := modeKeys + "\t\t" + strconv.Itoa(int(listen.Port))
				if mode != app.TunnelDynamic {
					fields += "\t" + destination.Host + "\t" + strconv.Itoa(int(destination.Port))
				}
				fields += "\x13"
				trafficDone := make(chan struct{})
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				input := pipeInput(t, []readerStage{{wait: writer.signal("q Quit  |  ? Help"), data: "l"}, {wait: writer.signal("Connection"), data: "p"}, {wait: writer.signal("Mode: local"), data: fields}, {wait: writer.signal("Start forwarding?"), data: "y"}, {wait: trafficDone, data: "\r"}, {wait: writer.signal("s Stop"), data: "s"}, {wait: writer.signal("Stop this tunnel?"), data: "y"}, {wait: writer.signal("Stopped"), data: "q"}})
				local := terminal.NewFake(size)
				type runResult struct {
					result tui.Result
					err    error
				}
				done := make(chan runResult, 1)
				joined := make(chan struct{})
				go func() {
					defer close(joined)
					result, err := tui.Run(ctx, tui.Config{Connections: fixture.connections, Folders: fixture.folders, Tunnels: fixture.service, Terminal: local, Stdin: input, Stdout: writer, Width: size.Columns, Height: size.Rows, NoColor: true})
					done <- runResult{result, err}
				}()
				t.Cleanup(func() {
					cancel()
					select {
					case <-joined:
					case <-time.After(3 * time.Second):
						t.Error("TUI worker did not join")
					}
				})
				select {
				case <-writer.signal("Tunnels (1 active)"):
				case <-ctx.Done():
					t.Fatalf("form did not activate forwarding: %s", writer.String())
				}
				config := app.TunnelConfig{Mode: mode, Listen: listen, Destination: destination}
				if err := forwardingExchange(forwardingDial(t, config, destination), []byte("TUI-created-forwarding")); err != nil {
					t.Fatal(err)
				}
				close(trafficDone)
				select {
				case result := <-done:
					if result.err != nil || result.result.RemoteExitStatus != nil {
						t.Fatalf("TUI creation failed: %v\n%s", result.err, writer.String())
					}
				case <-ctx.Done():
					t.Fatalf("form did not stop and quit: %s", writer.String())
				}
				snapshots := fixture.service.Snapshots()
				if len(snapshots) != 1 || snapshots[0].State != app.TunnelStopped || snapshots[0].Config.Mode != mode || snapshots[0].Connection.ID != connection.ID {
					t.Fatalf("wrong confirmed tunnel: %+v", snapshots)
				}
				if mode == app.TunnelDynamic && snapshots[0].Config.Destination != (app.TunnelEndpoint{}) {
					t.Fatal("dynamic request retained fixed destination")
				}
				current, err := fixture.connections.Get(context.Background(), app.ItemSelector{ID: connection.ID})
				if err != nil || current.Connection.Revision != connection.Revision {
					t.Fatal("forwarding mutated catalog")
				}
				forwardingPortReleased(t, listen)
				assertTerminalRestored(t, local)
			})
		}
	}
}
