package cli

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"

	"github.com/pluque01/orza/internal/app"
	"github.com/spf13/cobra"
)

func newConnectCommand(connections *app.ConnectionService, service *app.ConnectService, local app.Terminal, options *Options) *cobra.Command {
	return &cobra.Command{
		Use: "connect PATH_OR_ID", Short: "Open an interactive SSH session", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if local == nil || !local.Interactive() {
				return NewError(CodeSecurity, "connect requires an interactive terminal", args[0], app.ErrNonInteractive)
			}
			selector, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid connection path or ID", args[0], err)
			}
			if connections == nil || service == nil {
				return NewError(CodeCatalog, "connect service is unavailable", args[0], nil)
			}
			resolved, err := connections.Get(cmd.Context(), selector)
			if err != nil {
				return commandError(err, args[0])
			}
			target := net.JoinHostPort(resolved.Connection.Host, strconv.Itoa(int(resolved.Connection.Port)))
			if resolved.Connection.Username != "" {
				target = resolved.Connection.Username + "@" + target
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Connecting %s (%s)\n", resolved.Connection.Path, target); err != nil {
				return err
			}

			decide := func(_ context.Context, prompt app.TrustDecisionPrompt) (app.TrustDecision, error) {
				if _, err := fmt.Fprintf(cmd.OutOrStdout(), "Host key %s: %s %s\n", prompt.Status, prompt.Host.KeyAlgorithm, prompt.Host.FingerprintSHA256); err != nil {
					return app.TrustReject, err
				}
				if _, err := fmt.Fprint(cmd.OutOrStdout(), "Trust decision [reject/once/persist] (reject): "); err != nil {
					return app.TrustReject, err
				}
				answer, err := readLine(cmd.InOrStdin())
				if err != nil && answer == "" {
					return app.TrustReject, nil
				}
				switch strings.ToLower(strings.TrimSpace(answer)) {
				case "", "reject":
					return app.TrustReject, nil
				case "once", "trust once":
					return app.TrustOnce, nil
				case "persist", "trust and persist":
					return app.TrustPersist, nil
				default:
					return app.TrustReject, usageError("invalid host trust decision", resolved.Connection.Path, nil)
				}
			}
			expected := resolved.Connection.Revision
			captured := app.ItemSelector{ID: resolved.Connection.ID}
			result, err := service.Connect(cmd.Context(), app.ConnectRequest{Connection: captured, Expected: &expected, DecideTrust: decide})
			if result.Session.RemoteExitStatus != nil {
				return NewRemoteExitError(*result.Session.RemoteExitStatus)
			}
			if err != nil {
				endpoint := net.JoinHostPort(resolved.Connection.Host, strconv.Itoa(int(resolved.Connection.Port)))
				return connectCommandError(err, resolved.Connection.Path, endpoint, result.Session.Failure)
			}
			return nil
		},
	}
}

func connectCommandError(err error, target, endpoint string, diagnostic *app.SSHFailurePresentation) error {
	if diagnostic != nil {
		return newStartupError(codeForStartupDiagnostic(*diagnostic), diagnostic.Summary, target, endpoint, *diagnostic, err)
	}
	switch app.ErrorKindOf(err) {
	case app.ErrorKindInvalid:
		return NewError(CodeUsage, "connection request is invalid", target, err)
	case app.ErrorKindNotFound:
		return NewError(CodeNotFound, "connection was not found", target, err)
	case app.ErrorKindConflict:
		return NewError(CodeConflict, "host trust changed; retry", target, err)
	case app.ErrorKindCanceled:
		return NewError(CodeCanceled, "SSH session canceled", target, err)
	case app.ErrorKindSecurity:
		return NewError(CodeSecurity, "host trust or authentication failed", target, err)
	default:
		return NewError(CodeTransport, "SSH transport or protocol failed", target, err)
	}
}
