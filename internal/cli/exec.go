package cli

import (
	"context"
	"errors"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/spf13/cobra"
)

const defaultExecTimeout = 5 * time.Minute

func newExecCommand(service *app.CommandService, options *Options) *cobra.Command {
	var timeout time.Duration
	command := &cobra.Command{
		Use: "exec PATH_OR_ID -- COMMAND", Short: "Execute a remote command without a terminal",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 || strings.TrimSpace(args[1]) == "" {
				return NewError(CodeUsage, "exec requires a connection and one non-empty command after --", "", nil)
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if options.JSON {
				return NewError(CodeUsage, "--json cannot be used with exec", args[0], nil)
			}
			if timeout <= 0 {
				return NewError(CodeUsage, "--timeout must be positive", args[0], nil)
			}
			selector, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid connection path or ID", args[0], err)
			}
			if service == nil {
				return NewError(CodeCatalog, "exec service is unavailable", args[0], nil)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), timeout)
			defer cancel()
			result, err := service.Run(ctx, app.CommandRequest{Connection: selector, Command: args[1], Stdin: cmd.InOrStdin(), Stdout: cmd.OutOrStdout(), Stderr: cmd.ErrOrStderr()})
			if result.Session.RemoteExitStatus != nil {
				return NewRemoteExitError(*result.Session.RemoteExitStatus)
			}
			if err == nil {
				return nil
			}
			endpoint := net.JoinHostPort(result.Attempt.Host, strconv.Itoa(int(result.Attempt.Port)))
			if result.Session.Failure != nil {
				return newStartupError(codeForStartupDiagnostic(*result.Session.Failure), result.Session.Failure.Summary, result.Connection.Path, endpoint, *result.Session.Failure, err)
			}
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return NewError(CodeTransport, "remote command timed out", result.Connection.Path, err)
			}
			return execCommandError(err, args[0])
		},
	}
	command.Flags().DurationVar(&timeout, "timeout", defaultExecTimeout, "maximum command duration")
	return command
}

func execCommandError(err error, target string) error {
	switch app.ErrorKindOf(err) {
	case app.ErrorKindInvalid:
		return NewError(CodeUsage, "command request is invalid", target, err)
	case app.ErrorKindNotFound:
		return NewError(CodeNotFound, "connection was not found", target, err)
	case app.ErrorKindConflict:
		return NewError(CodeConflict, "the connection changed; retry", target, err)
	case app.ErrorKindCanceled:
		return NewError(CodeCanceled, "remote command canceled", target, err)
	case app.ErrorKindSecurity:
		return NewError(CodeSecurity, "host identity approval or non-interactive authentication is required", target, err)
	default:
		return NewError(CodeTransport, "SSH transport or command execution failed", target, err)
	}
}
