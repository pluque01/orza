package integration

import (
	"context"
	"fmt"
	"io"
	"os"
	"testing"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/sshclient"
	"github.com/pluque01/orza/internal/terminal"
	"github.com/pluque01/orza/internal/tui"
	"golang.org/x/crypto/ssh"
	"golang.org/x/sys/unix"
)

func TestTUIConsumesFirstKeyAfterInteractiveShell(t *testing.T) {
	t.Run("clean exit", func(t *testing.T) {
		assertFirstKeyAfterInteractiveShell(t, true, "Shell closed")
	})
	t.Run("missing exit status", func(t *testing.T) {
		assertFirstKeyAfterInteractiveShell(t, false, "Shell connection ended; browser restored.")
	})
}

func assertFirstKeyAfterInteractiveShell(t *testing.T, sendExitStatus bool, returnStatus string) {
	t.Helper()
	const shellReady = "Disposable shell ready"
	server := startForwardingServer(t, forwardingServerConfig{Session: func(_ context.Context, channel ssh.Channel, requests <-chan *ssh.Request) {
		for request := range requests {
			switch request.Type {
			case "pty-req":
				_ = request.Reply(true, nil)
			case "shell":
				_ = request.Reply(true, nil)
				_, _ = io.WriteString(channel, shellReady+"\n")
				var key [1]byte
				if _, err := io.ReadFull(channel, key[:]); err != nil || key[0] != 'x' {
					return
				}
				if sendExitStatus {
					_, _ = channel.SendRequest("exit-status", false, ssh.Marshal(struct{ Status uint32 }{0}))
				}
				return
			default:
				_ = request.Reply(false, nil)
			}
		}
	}})
	fixture := newForwardingAppFixture(t, "")
	fixture.create(t, server, "shell-host")
	fixture.trust(t, server)
	keyboard, input := openShellTestPTY(t)
	local := terminal.New(input, input)
	writer := newSignalingWriter("[ssh] shell-host", "Endpoint", "Connect to SSH target?", shellReady, returnStatus)
	options := fixture.options
	connect, err := app.NewConnectService(app.ConnectOptions{
		Connections: options.Connections, Credentials: options.Credentials, HostTrust: options.HostTrust,
		TrustedHosts: options.TrustedHosts, Store: options.Store, Scope: options.Scope, Terminal: local,
		Runner: sshclient.New(sshclient.Options{Stdin: input, Stdout: writer, Stderr: writer}),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	done := make(chan error, 1)
	joined := make(chan struct{})
	go func() {
		defer close(joined)
		_, err := tui.Run(ctx, tui.Config{Connections: fixture.connections, Folders: fixture.folders, Connect: connect, Terminal: local, Stdin: input, Stdout: writer, NoColor: true})
		done <- err
	}()
	t.Cleanup(func() {
		cancel()
		select {
		case <-joined:
		case <-time.After(3 * time.Second):
			t.Error("TUI did not stop after cancellation")
		}
	})
	for _, step := range []struct{ frame, key string }{
		{"[ssh] shell-host", "l"}, {"Endpoint", "c"}, {"Connect to SSH target?", "y"},
		{shellReady, "x"}, {returnStatus, "q"},
	} {
		select {
		case <-writer.signal(step.frame):
		case err := <-done:
			t.Fatalf("TUI stopped before %q: %v", step.frame, err)
		case <-ctx.Done():
			t.Fatalf("did not see %q: %s", step.frame, writer.String())
		}
		if _, err := io.WriteString(keyboard, step.key); err != nil {
			t.Fatal(err)
		}
	}
	select {
	case err := <-done:
		if err != nil || ctx.Err() != nil {
			t.Fatalf("TUI did not quit on the first key after shell return: %v, %v", err, ctx.Err())
		}
	case <-ctx.Done():
		t.Fatal("TUI did not quit on the first key after shell return")
	}
}

func openShellTestPTY(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	keyboard, err := os.OpenFile("/dev/ptmx", os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = keyboard.Close() })
	fd := int(keyboard.Fd())
	if err := unix.IoctlSetPointerInt(fd, unix.TIOCSPTLCK, 0); err != nil {
		t.Fatal(err)
	}
	number, err := unix.IoctlGetInt(fd, unix.TIOCGPTN)
	if err != nil {
		t.Fatal(err)
	}
	input, err := os.OpenFile(fmt.Sprintf("/dev/pts/%d", number), os.O_RDWR|unix.O_NOCTTY, 0)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = input.Close() })
	if err := unix.IoctlSetWinsize(int(input.Fd()), unix.TIOCSWINSZ, &unix.Winsize{Col: 80, Row: 24}); err != nil {
		t.Fatal(err)
	}
	return keyboard, input
}
