package cli

import (
	"bytes"
	"context"
	"os"
	"strings"
	"testing"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
	"github.com/pluque01/orza/internal/tui"
	"github.com/spf13/cobra"
)

func TestRootHelpAndVersion(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want string
	}{
		{name: "help", args: []string{"--help"}, want: "Usage:\n  orza [flags]"},
		{name: "version", args: []string{"--version"}, want: "orza 1.2.3\n"},
		{name: "version command", args: []string{"version"}, want: "orza 1.2.3\n"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			root := NewRoot(RootConfig{Version: "1.2.3", Stdin: strings.NewReader(""), Stdout: &stdout, Stderr: &stderr})
			root.SetArgs(tt.args)
			if err := root.ExecuteContext(context.Background()); err != nil {
				t.Fatalf("ExecuteContext() error = %v", err)
			}
			if !strings.Contains(stdout.String(), tt.want) {
				t.Fatalf("stdout = %q, want it to contain %q", stdout.String(), tt.want)
			}
			if stderr.Len() != 0 {
				t.Fatalf("stderr = %q, want empty", stderr.String())
			}
		})
	}
}

func TestRootPersistentOptionsApplyToStoryCommands(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	var stdout bytes.Buffer
	options := &Options{}
	root := NewRoot(RootConfig{Version: "dev", Stdout: &stdout, Stderr: &bytes.Buffer{}, Options: options})
	root.AddCommand(&cobra.Command{
		Use: "story",
		RunE: func(_ *cobra.Command, _ []string) error {
			if !options.JSON || !options.NoColor {
				t.Fatalf("options = %+v, want JSON and NoColor enabled", options)
			}
			return nil
		},
	})
	root.SetArgs([]string{"--json", "story"})

	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("ExecuteContext() error = %v", err)
	}
}

func TestRootNoColorEnvironmentAndFlagReachTUI(t *testing.T) {
	for _, tt := range []struct {
		name string
		env  *string
		args []string
	}{
		{name: "empty NO_COLOR is present", env: new(string)},
		{name: "no-color flag", args: []string{"--no-color"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if tt.env != nil {
				t.Setenv("NO_COLOR", *tt.env)
			} else {
				t.Setenv("NO_COLOR", "")
				if err := os.Unsetenv("NO_COLOR"); err != nil {
					t.Fatal(err)
				}
			}
			local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
			called := false
			root := NewRoot(RootConfig{
				Version: "dev", Terminal: local, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
				RunTUI: func(_ context.Context, config tui.Config) (tui.Result, error) {
					called = true
					if !config.NoColor {
						t.Fatal("TUI did not receive no-color mode")
					}
					return tui.Result{}, nil
				},
			})
			root.SetArgs(tt.args)
			if err := root.ExecuteContext(context.Background()); err != nil {
				t.Fatal(err)
			}
			if !called {
				t.Fatal("interactive invocation did not launch TUI")
			}
		})
	}
}

func TestRootWithoutCommandReturnsUsageAfterHelp(t *testing.T) {
	var stdout bytes.Buffer
	root := NewRoot(RootConfig{Version: "dev", Stdout: &stdout, Stderr: &bytes.Buffer{}})
	root.SetArgs(nil)

	err := root.ExecuteContext(context.Background())
	if got := ExitCode(err); got != ExitUsage {
		t.Fatalf("ExitCode(error) = %d, want %d (error %v)", got, ExitUsage, err)
	}
	if !strings.Contains(stdout.String(), "Usage:") {
		t.Fatalf("stdout = %q, want help", stdout.String())
	}
}

func TestRootWithoutCommandLaunchesTUIOnlyWhenInteractive(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	called := false
	root := NewRoot(RootConfig{
		Version: "dev", Terminal: local, Stdin: strings.NewReader(""), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{},
		RunTUI: func(_ context.Context, config tui.Config) (tui.Result, error) {
			called = true
			if config.Terminal != local || config.NoColor {
				t.Fatalf("TUI config = %+v", config)
			}
			return tui.Result{}, nil
		},
	})
	root.SetArgs(nil)
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("interactive no-command invocation did not launch TUI")
	}

	local.SetInteractive(false)
	called = false
	root = NewRoot(RootConfig{Version: "dev", Terminal: local, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, RunTUI: func(context.Context, tui.Config) (tui.Result, error) {
		called = true
		return tui.Result{}, nil
	}})
	root.SetArgs(nil)
	if err := root.ExecuteContext(context.Background()); ExitCode(err) != ExitUsage {
		t.Fatalf("pipeline error = %v, exit %d", err, ExitCode(err))
	}
	if called {
		t.Fatal("non-interactive no-command invocation launched TUI")
	}
}

func TestRootPropagatesTUISessionStatus(t *testing.T) {
	local := terminal.NewFake(terminal.Size{Columns: 80, Rows: 24})
	status := 19
	root := NewRoot(RootConfig{Version: "dev", Terminal: local, Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}, RunTUI: func(context.Context, tui.Config) (tui.Result, error) {
		return tui.Result{Session: app.SSHSessionResult{RemoteExitStatus: &status}, RemoteExitStatus: &status}, nil
	}})
	root.SetArgs(nil)
	err := root.ExecuteContext(context.Background())
	if ExitCode(err) != status {
		t.Fatalf("TUI session exit = %d, want %d (%v)", ExitCode(err), status, err)
	}
}

func TestRootFlagErrorsAreUsageErrors(t *testing.T) {
	root := NewRoot(RootConfig{Version: "dev", Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}})
	root.SetArgs([]string{"--definitely-unknown"})
	if got := ExitCode(root.ExecuteContext(context.Background())); got != ExitUsage {
		t.Fatalf("unknown flag exit = %d, want %d", got, ExitUsage)
	}
}

func TestTUIRunErrorPreservesRequiredVTRecovery(t *testing.T) {
	t.Parallel()
	err := tuiRunError(terminal.ErrRequiredVT)
	if ExitCode(err) != ExitUsage {
		t.Fatalf("VT startup exit = %d, want %d", ExitCode(err), ExitUsage)
	}
	if !strings.Contains(err.Error(), "use a VT-capable terminal") {
		t.Fatalf("VT startup error = %q, want actionable recovery", err)
	}
}
