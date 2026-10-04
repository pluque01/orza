package tui

import (
	"context"
	"errors"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
)

// Result describes the browser session's exit. Completed shells never supply
// its process status; these fields remain empty when browsing resumes.
type Result struct {
	Session          app.SSHSessionResult
	RemoteExitStatus *int
}

// Run owns one Bubble Tea program and restores the caller's terminal snapshot
// on every return path.
func Run(ctx context.Context, config Config) (result Result, err error) {
	if ctx == nil {
		failure := errors.New("run TUI: nil context")
		if config.Tunnels != nil {
			failure = errors.Join(failure, config.Tunnels.Close())
		}
		return result, failure
	}
	if config.Tunnels != nil {
		stopWatcher := make(chan struct{})
		watcherDone := make(chan struct{})
		go func() {
			defer close(watcherDone)
			select {
			case <-ctx.Done():
				_ = config.Tunnels.Close()
			case <-stopWatcher:
			}
		}()
		defer func() { close(stopWatcher); err = errors.Join(err, config.Tunnels.Close()); <-watcherDone }()
	}
	if config.Terminal == nil || !config.Terminal.Interactive() {
		return result, app.ErrNonInteractive
	}
	sessionCtx, cancelSession := context.WithCancel(ctx)
	defer cancelSession()
	config.Context = sessionCtx
	state, err := config.Terminal.Capture(ctx)
	if err != nil {
		return result, err
	}
	defer func() {
		restoreErr := config.Terminal.Restore(context.WithoutCancel(ctx), state)
		if restoreErr != nil {
			err = errors.Join(err, restoreErr)
		}
	}()
	capability, err := config.Terminal.VTCapability(ctx)
	if err != nil {
		return result, err
	}
	if err := requireVT(capability); err != nil {
		return result, err
	}
	if config.Width == 0 || config.Height == 0 {
		size, sizeErr := config.Terminal.Size(ctx)
		if sizeErr != nil {
			return result, sizeErr
		}
		config.Width, config.Height = size.Columns, size.Rows
	}

	input := config.Stdin
	if input == nil {
		input = nil
	}
	output := config.Stdout
	if output == nil {
		output = io.Discard
	}
	model := New(config)
	defer model.releaseTunnelInput()
	program := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithWindowSize(config.Width, config.Height),
	)
	final, runErr := program.Run()
	cancelSession()
	if config.Tunnels != nil {
		runErr = errors.Join(runErr, config.Tunnels.Close())
	}
	if runErr != nil {
		return result, runErr
	}
	_, ok := final.(*Model)
	if !ok {
		return result, errors.New("run TUI: unexpected final model")
	}
	return result, nil
}

func requireVT(capability terminal.VTCapability) error {
	switch capability.Status {
	case terminal.VTSupported, terminal.VTUnverified:
		return nil
	case terminal.VTMissing:
		if capability.SafeFallback {
			return nil
		}
		return terminal.ErrRequiredVT
	default:
		return errors.New("run TUI: invalid VT capability report")
	}
}
