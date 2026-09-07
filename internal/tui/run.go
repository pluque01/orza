package tui

import (
	"context"
	"errors"
	"io"

	tea "charm.land/bubbletea/v2"
	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
)

// Result communicates an active SSH session outcome without coupling the TUI
// adapter to CLI exit-code policy.
type Result struct {
	Session          app.SSHSessionResult
	RemoteExitStatus *int
}

// Run owns one Bubble Tea program and restores the caller's terminal snapshot
// on every return path.
func Run(ctx context.Context, config Config) (result Result, err error) {
	if ctx == nil {
		return result, errors.New("run TUI: nil context")
	}
	if config.Terminal == nil || !config.Terminal.Interactive() {
		return result, app.ErrNonInteractive
	}
	config.Context = ctx
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
	program := tea.NewProgram(model,
		tea.WithContext(ctx),
		tea.WithInput(input),
		tea.WithOutput(output),
		tea.WithWindowSize(config.Width, config.Height),
	)
	final, runErr := program.Run()
	if runErr != nil {
		return result, runErr
	}
	root, ok := final.(*Model)
	if !ok {
		return result, errors.New("run TUI: unexpected final model")
	}
	result.Session = root.sessionResult.Session
	result.RemoteExitStatus = root.sessionResult.Session.RemoteExitStatus
	if root.sessionErr != nil && sessionWasActive(root.sessionResult) {
		return result, root.sessionErr
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
