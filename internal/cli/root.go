package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
	"github.com/pluque01/orza/internal/tui"
	"github.com/spf13/cobra"
)

// Options contains process-wide CLI presentation settings shared by story commands.
type Options struct {
	JSON    bool
	NoColor bool
}

// RootConfig supplies process dependencies and streams without global state.
type RootConfig struct {
	Version      string
	Dependencies *app.Dependencies
	Connections  *app.ConnectionService
	Folders      *app.FolderService
	Connect      *app.ConnectService
	Terminal     app.Terminal
	Stdin        io.Reader
	Stdout       io.Writer
	Stderr       io.Writer
	Options      *Options
	RunTUI       func(context.Context, tui.Config) (tui.Result, error)
}

// NewRoot constructs the command tree root. Story packages can extend the
// returned command with children that share Config dependencies and options.
func NewRoot(config RootConfig) *cobra.Command {
	options := config.Options
	if options == nil {
		options = &Options{}
	}
	if _, set := os.LookupEnv("NO_COLOR"); set {
		options.NoColor = true
	}
	connections := config.Connections
	folders := config.Folders
	connectService := config.Connect
	localTerminal := config.Terminal
	if config.Dependencies != nil {
		if connections == nil {
			connections = config.Dependencies.Connections
		}
		if folders == nil {
			folders = config.Dependencies.Folders
		}
		if connectService == nil {
			connectService = config.Dependencies.Connect
		}
		if localTerminal == nil {
			localTerminal = config.Dependencies.Terminal
		}
	}

	root := &cobra.Command{
		Use:           "orza",
		Short:         "Manage and open SSH connections",
		Version:       config.Version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return NewError(CodeUsage, "invalid command usage", "", nil)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if options.JSON || localTerminal == nil || !localTerminal.Interactive() {
				if err := cmd.Help(); err != nil {
					return err
				}
				return NewError(CodeUsage, "an interactive terminal is required when no command is given", "", app.ErrNonInteractive)
			}
			run := config.RunTUI
			if run == nil {
				run = tui.Run
			}
			result, err := run(cmd.Context(), tui.Config{
				Connections: connections, Folders: folders, Connect: connectService, Terminal: localTerminal,
				Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr(),
				NoColor: options.NoColor,
			})
			if result.RemoteExitStatus != nil {
				return NewRemoteExitError(*result.RemoteExitStatus)
			}
			if err != nil {
				return tuiRunError(err)
			}
			return nil
		},
	}
	root.SetVersionTemplate("orza {{.Version}}\n")
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return NewError(CodeUsage, "invalid command usage", "", err)
	})
	root.PersistentFlags().BoolVar(&options.JSON, "json", options.JSON, "write one machine-readable JSON response")
	root.PersistentFlags().BoolVar(&options.NoColor, "no-color", options.NoColor, "disable color output")
	root.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "orza %s\n", config.Version)
			return err
		},
	})

	connection := newConnectionCommand(connections, localTerminal, options)
	connection.AddCommand(newConnectionCreateCommand(connections, localTerminal, options))
	connection.AddCommand(newConnectionListCommand(connections, options))
	connection.AddCommand(newConnectionShowCommand(connections, options))
	connection.AddCommand(newConnectionUpdateCommand(connections, localTerminal, options))
	connection.AddCommand(newConnectionMoveCommand(connections, options))
	connection.AddCommand(newConnectionDeleteCommand(connections, localTerminal, options))
	root.AddCommand(connection, newConnectCommand(connections, connectService, localTerminal, options))
	folder := newFolderCommand()
	folder.AddCommand(newFolderCreateCommand(folders, options))
	folder.AddCommand(newFolderListCommand(folders, options))
	folder.AddCommand(newFolderShowCommand(folders, options))
	folder.AddCommand(newFolderRenameCommand(folders, options))
	folder.AddCommand(newFolderMoveCommand(folders, options))
	folder.AddCommand(newFolderDeleteCommand(folders, localTerminal, options))
	root.AddCommand(folder)

	if config.Stdin != nil {
		root.SetIn(config.Stdin)
	}
	if config.Stdout != nil {
		root.SetOut(config.Stdout)
	}
	if config.Stderr != nil {
		root.SetErr(config.Stderr)
	}
	return root
}

func tuiRunError(err error) error {
	if errors.Is(err, terminal.ErrRequiredVT) {
		return NewError(CodeUsage, terminal.ErrRequiredVT.Error(), "", err)
	}
	switch app.ErrorKindOf(err) {
	case app.ErrorKindInvalid:
		return NewError(CodeUsage, "interactive interface request is invalid", "", err)
	case app.ErrorKindNotFound:
		return NewError(CodeNotFound, "connection was not found", "", err)
	case app.ErrorKindConflict:
		return NewError(CodeConflict, "the connection changed; reload before retrying", "", err)
	case app.ErrorKindCanceled:
		return NewError(CodeCanceled, "interactive interface canceled", "", err)
	case app.ErrorKindSecurity:
		return NewError(CodeSecurity, "host trust or authentication failed", "", err)
	default:
		return NewError(CodeTransport, "interactive session failed", "", err)
	}
}

func newConnectionCommand(_ *app.ConnectionService, _ app.Terminal, _ *Options) *cobra.Command {
	return &cobra.Command{
		Use:   "connection",
		Short: "Manage SSH connections",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := cmd.Help(); err != nil {
				return err
			}
			return NewError(CodeUsage, "a connection command is required", "", nil)
		},
	}
}
