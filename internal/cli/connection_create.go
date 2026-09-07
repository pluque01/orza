package cli

import (
	"context"
	"fmt"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/domain"
	"github.com/spf13/cobra"
)

func newConnectionCreateCommand(service *app.ConnectionService, local app.Terminal, options *Options) *cobra.Command {
	var host, user, auth, identity string
	var port uint16
	var remember bool
	command := &cobra.Command{
		Use:   "create PATH",
		Short: "Create a connection",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if service == nil {
				return NewError(CodeCatalog, "connection service is unavailable", args[0], nil)
			}
			parent, name, err := splitConnectionPath(args[0])
			if err != nil {
				return usageError("invalid connection path", args[0], err)
			}
			method, err := parseAuthMethod(auth)
			if err != nil || host == "" || port == 0 {
				return usageError("host, port, and authentication method must be valid", args[0], err)
			}
			if err := validateAuthFlags(method, identity); err != nil {
				return usageError("authentication flags are inconsistent", args[0], err)
			}
			if remember && method != app.AuthMethodPassword {
				return usageError("--remember-password requires --auth password", args[0], nil)
			}

			request := app.CreateConnectionRequest{
				Parent: app.ItemSelector{Path: parent}, Name: name, Host: host, Port: port,
				Username: user, AuthMethod: method, IdentityFile: identity,
			}
			if remember {
				secret, promptErr := readRememberedPassword(cmd.Context(), cmd, local)
				if promptErr != nil {
					return promptErr
				}
				defer wipe(secret)
				request.CredentialIntent = app.CredentialRemember
				request.Password = secret
			}
			result, err := service.Create(cmd.Context(), request)
			if err != nil {
				return commandError(err, args[0])
			}
			return writeConnectionResult(cmd, options.JSON, result, "created")
		},
	}
	command.Flags().StringVar(&host, "host", "", "SSH host")
	command.Flags().Uint16Var(&port, "port", domain.DefaultSSHPort, "SSH port")
	command.Flags().StringVar(&user, "user", "", "SSH username")
	command.Flags().StringVar(&auth, "auth", "", "authentication method: agent, key, or password")
	command.Flags().StringVar(&identity, "identity-file", "", "private key path")
	command.Flags().BoolVar(&remember, "remember-password", false, "prompt for and securely remember the password")
	return command
}

func splitConnectionPath(value string) (string, string, error) {
	path, err := domain.ParseLogicalPath(value)
	if err != nil || path.IsRoot() {
		return "", "", domain.ErrInvalidPath
	}
	segments := path.Segments()
	name := segments[len(segments)-1].String()
	parent, err := domain.NewLogicalPath(segments[:len(segments)-1]...)
	if err != nil {
		return "", "", err
	}
	return parent.String(), name, nil
}

func parseAuthMethod(value string) (app.AuthMethod, error) {
	switch app.AuthMethod(value) {
	case app.AuthMethodAgent, app.AuthMethodKey, app.AuthMethodPassword:
		return app.AuthMethod(value), nil
	default:
		return "", domain.ErrInvalidAuthMethod
	}
}

func validateAuthFlags(method app.AuthMethod, identity string) error {
	if method == app.AuthMethodKey && identity == "" {
		return domain.ErrInvalidIdentityFile
	}
	if method != app.AuthMethodKey && identity != "" {
		return domain.ErrInvalidAuthentication
	}
	return nil
}

func readRememberedPassword(ctx context.Context, cmd *cobra.Command, local app.Terminal) ([]byte, error) {
	if local == nil || !local.Interactive() {
		return nil, NewError(CodeSecurity, "remembering a password requires an interactive terminal", "", app.ErrNonInteractive)
	}
	confirmed, err := confirm(cmd, "Remember password in the operating system credential store? [y/N] ")
	if err != nil {
		return nil, err
	}
	if !confirmed {
		return nil, NewError(CodeCanceled, "password storage was not approved", "", context.Canceled)
	}
	secret, err := local.ReadSecret(ctx, terminalSecretPrompt("Password: "))
	if err != nil {
		wipe(secret)
		return nil, NewError(CodeSecurity, "password could not be read securely", "", err)
	}
	return secret, nil
}

func writeConnectionResult(cmd *cobra.Command, jsonOutput bool, result app.ConnectionResult, verb string) error {
	projection := projectConnection(result.Connection)
	if jsonOutput {
		return WriteSuccess(cmd.OutOrStdout(), true, projection, &result.CatalogRevision)
	}
	return WriteSuccess(cmd.OutOrStdout(), false, fmt.Sprintf("%s %s (%s), revision %d", verb, result.Connection.Path, result.Connection.ID, result.Connection.Revision), nil)
}
