//go:build !windows

package integration

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/pluque01/orza/internal/terminal"
)

func TestTerminalSecretPTYPromptPasteAndSelection(t *testing.T) {
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script(1) is required for PTY integration")
	}
	tests := []struct {
		name      string
		prompt    string
		input     string
		expected  string
		wantBytes int
	}{
		{name: "remembered-password", prompt: "Remember password: ", input: "ab\x1b[1;2D\x1b[200~界\x1b[201~\r", expected: "a界", wantBytes: 4},
		{name: "ssh-password", prompt: "Password: ", input: "\x1b[200~a\r\nb\u0085c\u2028d\u2029e\x1b[201~\r", expected: "abcde", wantBytes: 5},
		{name: "private-key-passphrase", prompt: "Private key passphrase: ", input: "ok\x1b[200~SECRET_CANARY_002\t\x1b[201~\r", expected: "ok", wantBytes: 2},
		{name: "control-byte-paste", prompt: "Password: ", input: "safe\x1b[200~REJECTED\x00\x07\x1b[31m\x7f\u0080CONTROL_CANARY\x1b[201~z\r", expected: "safez", wantBytes: 5},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			output, err := runSecretPTY(t, map[string]string{
				"ORZA_SECRET_HELPER":   "prompt",
				"ORZA_SECRET_PROMPT":   test.prompt,
				"ORZA_SECRET_EXPECTED": test.expected,
			}, test.input)
			if err != nil {
				t.Fatalf("PTY helper failed: %v", err)
			}
			if !strings.Contains(output, fmt.Sprintf("RESULT:%d", test.wantBytes)) || !strings.Contains(output, "MATCH") {
				t.Fatal("PTY helper returned an unexpected secret length")
			}
			if strings.Contains(output, "SECRET_CANARY_002") || strings.Contains(output, "CONTROL_CANARY") {
				t.Fatal("PTY output exposed secret input")
			}
			if !strings.Contains(output, ansi.SetModeBracketedPaste) || !strings.Contains(output, ansi.ResetModeBracketedPaste) ||
				!strings.Contains(output, "RESTORED") {
				t.Fatal("PTY prompt did not enable/disable paste and restore terminal state")
			}
		})
	}
}

func TestTerminalSecretPTYContextCleanup(t *testing.T) {
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script(1) is required for PTY integration")
	}
	for _, mode := range []string{"cancel", "timeout"} {
		t.Run(mode, func(t *testing.T) {
			output, command, input := startSecretPTY(t, map[string]string{
				"ORZA_SECRET_HELPER": mode,
				"ORZA_SECRET_PROMPT": "Password: ",
			}, "")
			defer input.Close()
			waitForOutput(t, output, ansi.SetModeBracketedPaste)
			if err := command.Wait(); err != nil {
				t.Fatalf("%s helper failed: %v", mode, err)
			}
			captured := output.String()
			if !strings.Contains(captured, strings.ToUpper(mode)+"_CLEAN") ||
				!strings.Contains(captured, ansi.ResetModeBracketedPaste) || !strings.Contains(captured, "RESTORED") {
				t.Fatalf("%s completed before prompt cleanup", mode)
			}
		})
	}
}

func TestTerminalSecretPTYSignalCleanup(t *testing.T) {
	if _, err := exec.LookPath("script"); err != nil {
		t.Skip("script(1) is required for PTY integration")
	}
	tests := []struct {
		name     string
		signal   syscall.Signal
		wantExit int
	}{
		{name: "interrupt", signal: syscall.SIGINT, wantExit: 130},
		{name: "term", signal: syscall.SIGTERM, wantExit: 143},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pidFile := t.TempDir() + "/pid"
			output, command, input := startSecretPTY(t, map[string]string{
				"ORZA_SECRET_HELPER": "signal",
				"ORZA_SECRET_PROMPT": "Password: ",
			}, pidFile)
			defer input.Close()
			waitForOutput(t, output, ansi.SetModeBracketedPaste)
			pidBytes, err := os.ReadFile(pidFile)
			if err != nil {
				t.Fatal(err)
			}
			pid, err := strconv.Atoi(strings.TrimSpace(string(pidBytes)))
			if err != nil {
				t.Fatal(err)
			}
			if err := syscall.Kill(pid, test.signal); err != nil {
				t.Fatal(err)
			}
			err = command.Wait()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != test.wantExit {
				t.Fatalf("signal helper exit = %v, want %d", err, test.wantExit)
			}
			captured := output.String()
			if !strings.Contains(captured, "SIGNAL_CLEAN") || !strings.Contains(captured, ansi.ResetModeBracketedPaste) {
				t.Fatal("signal exit happened before prompt cleanup")
			}
		})
	}
}

func TestTerminalSecretPTYHelper(t *testing.T) {
	mode := os.Getenv("ORZA_SECRET_HELPER")
	if mode == "" {
		return
	}
	system := terminal.New(os.Stdin, os.Stdout)
	ctx := context.Background()
	var signalCode chan int
	var contextErr error
	var contextTimer *time.Timer
	if mode == "signal" {
		cancelCtx, cancel := context.WithCancel(ctx)
		ctx = cancelCtx
		signalCode = make(chan int, 1)
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
		go func() {
			received := <-signals
			signal.Stop(signals)
			if received == syscall.SIGTERM {
				signalCode <- 143
			} else {
				signalCode <- 130
			}
			cancel()
		}()
	} else if mode == "cancel" {
		cancelCtx, cancel := context.WithCancel(ctx)
		ctx = cancelCtx
		contextErr = context.Canceled
		contextTimer = time.AfterFunc(100*time.Millisecond, cancel)
		defer cancel()
	} else if mode == "timeout" {
		timeoutCtx, cancel := context.WithTimeout(ctx, 100*time.Millisecond)
		ctx = timeoutCtx
		contextErr = context.DeadlineExceeded
		defer cancel()
	}
	secret, err := system.ReadSecret(ctx, terminal.SecretPrompt{Message: os.Getenv("ORZA_SECRET_PROMPT")})
	if mode == "signal" {
		if !errors.Is(err, context.Canceled) || secret != nil {
			t.Fatal("signal did not cancel secret input")
		}
		state, captureErr := system.Capture(context.Background())
		if captureErr != nil || state.Raw() {
			t.Fatal("terminal remained raw after signal")
		}
		fmt.Fprint(os.Stdout, "SIGNAL_CLEAN\n")
		os.Exit(<-signalCode)
	}
	if contextErr != nil {
		if contextTimer != nil {
			contextTimer.Stop()
		}
		if !errors.Is(err, contextErr) || secret != nil {
			t.Fatalf("%s did not cancel secret input: %v", mode, err)
		}
		state, captureErr := system.Capture(context.Background())
		if captureErr != nil || state.Raw() {
			t.Fatalf("terminal remained raw after %s", mode)
		}
		fmt.Fprintf(os.Stdout, "%s_CLEAN\nRESTORED\n", strings.ToUpper(mode))
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(secret, []byte(os.Getenv("ORZA_SECRET_EXPECTED"))) {
		t.Fatal("secret editor result did not match expected manual/paste editing")
	}
	length := len(secret)
	for index := range secret {
		secret[index] = 0
	}
	state, err := system.Capture(context.Background())
	if err != nil || state.Raw() {
		t.Fatal("terminal remained raw after submit")
	}
	fmt.Fprintf(os.Stdout, "RESULT:%d\nMATCH\nRESTORED\n", length)
}

func runSecretPTY(t *testing.T, env map[string]string, inputValue string) (string, error) {
	t.Helper()
	output, command, input := startSecretPTY(t, env, "")
	waitForOutput(t, output, ansi.SetModeBracketedPaste)
	if _, err := input.Write([]byte(inputValue)); err != nil {
		return output.String(), err
	}
	_ = input.Close()
	err := command.Wait()
	return output.String(), err
}

func startSecretPTY(t *testing.T, env map[string]string, pidFile string) (*lockedBuffer, *exec.Cmd, ioWriteCloser) {
	t.Helper()
	commandLine := "exec " + strconv.Quote(os.Args[0]) + " -test.run=^TestTerminalSecretPTYHelper$"
	if pidFile != "" {
		commandLine = "echo $$ > " + strconv.Quote(pidFile) + "; " + commandLine
	}
	command := exec.Command("script", "-q", "-e", "-c", commandLine, "/dev/null")
	command.Env = append(os.Environ(), "TERM=xterm-256color")
	for name, value := range env {
		command.Env = append(command.Env, name+"="+value)
	}
	input, err := command.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	output := new(lockedBuffer)
	command.Stdout = output
	command.Stderr = output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if command.ProcessState == nil {
			_ = command.Process.Kill()
			_ = command.Wait()
		}
	})
	return output, command, input
}

func waitForOutput(t *testing.T, output *lockedBuffer, value string) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(output.String(), value) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for PTY prompt")
}

type ioWriteCloser interface {
	Write([]byte) (int, error)
	Close() error
}

type lockedBuffer struct {
	mu     sync.Mutex
	buffer bytes.Buffer
}

func (b *lockedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	const maxCapturedBytes = 1 << 20
	remaining := maxCapturedBytes - b.buffer.Len()
	if remaining > 0 {
		_, _ = b.buffer.Write(value[:min(remaining, len(value))])
	}
	return len(value), nil
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buffer.String()
}
