package cli

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/domain"
	"github.com/pluque01/orza/internal/terminal"
	"github.com/spf13/cobra"
)

func newConnectionUpdateCommand(service *app.ConnectionService, local app.Terminal, options *Options) *cobra.Command {
	var name, host, user, auth, identity string
	var port uint16
	var clearUser, clearIdentity, remember, forget bool
	var expected uint64
	command := &cobra.Command{
		Use:   "update PATH_OR_ID",
		Short: "Update a connection",
		Args:  exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selector, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid connection path or ID", args[0], err)
			}
			flags := cmd.Flags()
			if flags.Changed("user") && clearUser || flags.Changed("identity-file") && clearIdentity || remember && forget {
				return usageError("mutually exclusive update flags were combined", args[0], nil)
			}
			if !flags.Changed("name") && !flags.Changed("host") && !flags.Changed("port") && !flags.Changed("user") && !clearUser && !flags.Changed("auth") && !flags.Changed("identity-file") && !clearIdentity && !remember && !forget {
				return usageError("at least one update is required", args[0], nil)
			}
			request := app.UpdateConnectionRequest{Connection: selector}
			if flags.Changed("if-revision") {
				if expected == 0 {
					return usageError("revision must be greater than zero", args[0], domain.ErrInvalidRevision)
				}
				revision := app.Revision(expected)
				request.Expected = &revision
			}
			if flags.Changed("name") {
				request.Name = &name
			}
			if flags.Changed("host") {
				request.Host = &host
			}
			if flags.Changed("port") {
				request.Port = &port
			}
			if flags.Changed("user") {
				request.Username = &user
			}
			if clearUser {
				empty := ""
				request.Username = &empty
			}
			if flags.Changed("auth") {
				method, parseErr := parseAuthMethod(auth)
				if parseErr != nil {
					return usageError("invalid authentication method", args[0], parseErr)
				}
				request.AuthMethod = &method
			}
			if flags.Changed("identity-file") {
				request.IdentityFile = &identity
			}
			if clearIdentity {
				empty := ""
				request.IdentityFile = &empty
			}
			if remember {
				if service == nil {
					return NewError(CodeCatalog, "connection service is unavailable", args[0], nil)
				}
				current, getErr := service.Get(cmd.Context(), selector)
				if getErr != nil {
					return commandError(getErr, args[0])
				}
				if request.Expected != nil && *request.Expected != current.Connection.Revision {
					return NewError(CodeConflict, "the connection changed; reload before retrying", args[0], app.ErrConflict)
				}
				method := current.Connection.AuthMethod
				if request.AuthMethod != nil {
					method = *request.AuthMethod
				}
				finalIdentity := current.Connection.IdentityFile
				if request.IdentityFile != nil {
					finalIdentity = *request.IdentityFile
				}
				if method != app.AuthMethodPassword || finalIdentity != "" {
					return usageError("--remember-password requires password authentication without an identity file", args[0], nil)
				}
				secret, promptErr := readRememberedPassword(cmd.Context(), cmd, local)
				if promptErr != nil {
					return promptErr
				}
				defer wipe(secret)
				request.CredentialIntent, request.Password = app.CredentialRemember, secret
			}
			if forget {
				request.CredentialIntent = app.CredentialForget
			}
			if service == nil {
				return NewError(CodeCatalog, "connection service is unavailable", args[0], nil)
			}
			result, err := service.Update(cmd.Context(), request)
			if err != nil {
				return commandError(err, args[0])
			}
			return writeConnectionResult(cmd, options.JSON, result, "updated")
		},
	}
	command.Flags().StringVar(&name, "name", "", "new connection name")
	command.Flags().StringVar(&host, "host", "", "new SSH host")
	command.Flags().Uint16Var(&port, "port", 0, "new SSH port")
	command.Flags().StringVar(&user, "user", "", "new SSH username")
	command.Flags().BoolVar(&clearUser, "clear-user", false, "clear the SSH username")
	command.Flags().StringVar(&auth, "auth", "", "new authentication method")
	command.Flags().StringVar(&identity, "identity-file", "", "new private key path")
	command.Flags().BoolVar(&clearIdentity, "clear-identity-file", false, "clear the private key path")
	command.Flags().BoolVar(&remember, "remember-password", false, "prompt for and securely remember a password")
	command.Flags().BoolVar(&forget, "forget-password", false, "remove the remembered password")
	command.Flags().Uint64Var(&expected, "if-revision", 0, "require this connection revision")
	return command
}

func newConnectionMoveCommand(service *app.ConnectionService, options *Options) *cobra.Command {
	var expected uint64
	command := &cobra.Command{
		Use: "move PATH_OR_ID DESTINATION_FOLDER", Short: "Move a connection", Args: exactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			connection, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid connection path or ID", args[0], err)
			}
			destination, err := parseSelector(args[1])
			if err != nil {
				return usageError("invalid destination path or ID", args[1], err)
			}
			request := app.MoveConnectionRequest{Connection: connection, Destination: destination}
			if cmd.Flags().Changed("if-revision") {
				if expected == 0 {
					return usageError("revision must be greater than zero", args[0], domain.ErrInvalidRevision)
				}
				revision := app.Revision(expected)
				request.Expected = &revision
			}
			if service == nil {
				return NewError(CodeCatalog, "connection service is unavailable", args[0], nil)
			}
			result, err := service.Move(cmd.Context(), request)
			if err != nil {
				return commandError(err, args[0])
			}
			return writeConnectionResult(cmd, options.JSON, result, "moved")
		},
	}
	command.Flags().Uint64Var(&expected, "if-revision", 0, "require this connection revision")
	return command
}

func newConnectionDeleteCommand(service *app.ConnectionService, local app.Terminal, options *Options) *cobra.Command {
	var expected uint64
	var yes bool
	command := &cobra.Command{
		Use: "delete PATH_OR_ID", Short: "Delete a connection", Args: exactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			selector, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid connection path or ID", args[0], err)
			}
			if service == nil {
				return NewError(CodeCatalog, "connection service is unavailable", args[0], nil)
			}
			var revision *app.Revision
			if cmd.Flags().Changed("if-revision") {
				if expected == 0 {
					return usageError("revision must be greater than zero", args[0], domain.ErrInvalidRevision)
				}
				value := app.Revision(expected)
				revision = &value
			}
			if !yes {
				if local == nil || !local.Interactive() {
					return NewError(CodeSecurity, "deletion confirmation requires an interactive terminal or --yes", args[0], app.ErrNonInteractive)
				}
				scope, scopeErr := service.DeleteScope(cmd.Context(), selector)
				if scopeErr != nil {
					return commandError(scopeErr, args[0])
				}
				if revision != nil && *revision != scope.Revision {
					return NewError(CodeConflict, "the connection changed; reload before retrying", args[0], app.ErrConflict)
				}
				user := scope.Username
				if user == "" {
					user = "(default)"
				}
				prompt := fmt.Sprintf("Delete %s at %s (%s@%s)? [y/N] ", scope.Name, scope.Path, user, scope.Host)
				confirmed, confirmErr := confirm(cmd, prompt)
				if confirmErr != nil {
					return confirmErr
				}
				if !confirmed {
					return NewError(CodeCanceled, "deletion canceled", args[0], context.Canceled)
				}
				selector = app.ItemSelector{ID: scope.ID}
				pinned := scope.Revision
				revision = &pinned
			}
			result, err := service.Delete(cmd.Context(), app.DeleteConnectionRequest{Connection: selector, Expected: revision})
			if err != nil {
				return commandError(err, args[0])
			}
			data := struct {
				ID      app.NodeID `json:"id"`
				Path    string     `json:"path"`
				Deleted bool       `json:"deleted"`
			}{result.Deleted.ID, result.Deleted.Path, true}
			if options.JSON {
				return WriteSuccess(cmd.OutOrStdout(), true, data, &result.CatalogRevision)
			}
			return WriteSuccess(cmd.OutOrStdout(), false, fmt.Sprintf("deleted %s (%s)", result.Deleted.Path, result.Deleted.ID), nil)
		},
	}
	command.Flags().Uint64Var(&expected, "if-revision", 0, "require this connection revision")
	command.Flags().BoolVar(&yes, "yes", false, "delete without an interactive confirmation")
	return command
}

func exactArgs(count int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) != count {
			return NewError(CodeUsage, "invalid command usage", "", nil)
		}
		return nil
	}
}
func maximumArgs(count int) cobra.PositionalArgs {
	return func(_ *cobra.Command, args []string) error {
		if len(args) > count {
			return NewError(CodeUsage, "invalid command usage", "", nil)
		}
		return nil
	}
}
func usageError(message, target string, cause error) error {
	return NewError(CodeUsage, message, target, cause)
}

func confirm(cmd *cobra.Command, prompt string) (bool, error) {
	if _, err := io.WriteString(cmd.OutOrStdout(), prompt); err != nil {
		return false, err
	}
	line, err := readLine(cmd.InOrStdin())
	if err != nil && len(line) == 0 {
		return false, NewError(CodeCanceled, "confirmation was not provided", "", err)
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true, nil
	default:
		return false, nil
	}
}

func readLine(reader io.Reader) (string, error) {
	var result strings.Builder
	buffer := []byte{0}
	for {
		n, err := reader.Read(buffer)
		if n != 0 {
			if buffer[0] == '\n' {
				return strings.TrimSuffix(result.String(), "\r"), nil
			}
			result.WriteByte(buffer[0])
		}
		if err != nil {
			return result.String(), err
		}
	}
}

func commandError(err error, target string) error {
	code, message := CodeCatalog, "catalog operation failed"
	switch app.ErrorKindOf(err) {
	case app.ErrorKindInvalid:
		code, message = CodeUsage, "request is invalid"
	case app.ErrorKindNotFound:
		code, message = CodeNotFound, "connection was not found"
	case app.ErrorKindConflict:
		code, message = CodeConflict, "the connection changed; reload before retrying"
	case app.ErrorKindCanceled:
		code, message = CodeCanceled, "operation canceled"
	case app.ErrorKindSecurity:
		code, message = CodeSecurity, "secure operation failed"
	}
	return NewError(code, message, target, err)
}

func terminalSecretPrompt(message string) terminal.SecretPrompt {
	return terminal.SecretPrompt{Message: message}
}
func wipe(secret []byte) {
	for index := range secret {
		secret[index] = 0
	}
}
