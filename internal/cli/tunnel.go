package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/terminal"
	"github.com/spf13/cobra"
)

// TunnelPolicy is selected before dependency construction and cannot be mutated
// by command execution or credential recovery.
type TunnelPolicy struct{ tunnel, nonInteractive bool }

func (p TunnelPolicy) NonInteractive() bool { return p.nonInteractive }
func (p TunnelPolicy) IsTunnel() bool       { return p.tunnel }

// PreflightTunnel uses the production command tree and the same validation as
// execution, without resolving a catalog selector or acquiring credentials.
func PreflightTunnel(args []string, interactive bool) (TunnelPolicy, error) {
	options := &Options{}
	root := NewRoot(RootConfig{Options: options, Stdout: io.Discard, Stderr: io.Discard})
	root.InitDefaultHelpFlag()
	root.InitDefaultVersionFlag()
	cmd, rest, err := root.Find(args)
	if cmd == nil {
		return TunnelPolicy{}, nil
	}
	isTunnel := cmd.Name() == "tunnel" || cmd.Parent() != nil && cmd.Parent().Name() == "tunnel"
	if !isTunnel {
		return TunnelPolicy{}, nil
	}
	fail := func(err error) (TunnelPolicy, error) {
		var usage *ManagementError
		if errors.As(err, &usage) && usage.code == CodeUsage {
			return TunnelPolicy{tunnel: true}, err
		}
		return TunnelPolicy{tunnel: true}, usageError("invalid tunnel command or options", "", err)
	}
	if err != nil {
		return fail(err)
	}
	cmd.InitDefaultHelpFlag()
	if err := cmd.ParseFlags(rest); err != nil {
		return fail(err)
	}
	help, _ := cmd.Flags().GetBool("help")
	if help {
		return TunnelPolicy{tunnel: true}, nil
	}
	if cmd.Name() == "tunnel" {
		return fail(nil)
	}
	if err := cmd.ValidateArgs(cmd.Flags().Args()); err != nil {
		return fail(err)
	}
	_, unattended, err := tunnelFlags(cmd, options, interactive)
	if err != nil {
		return fail(err)
	}
	if _, err := parseSelector(cmd.Flags().Args()[0]); err != nil {
		return fail(err)
	}
	return TunnelPolicy{tunnel: true, nonInteractive: unattended}, nil
}

func newTunnelCommand(connections *app.ConnectionService, service *app.TunnelService, local app.Terminal, options *Options) *cobra.Command {
	parent := &cobra.Command{Use: "tunnel", Short: "Run one session-only foreground TCP tunnel", Args: cobra.NoArgs, RunE: func(*cobra.Command, []string) error { return usageError("a tunnel mode is required", "", nil) }}
	for _, mode := range []app.TunnelMode{app.TunnelLocal, app.TunnelRemote, app.TunnelDynamic} {
		cmd := &cobra.Command{Use: string(mode) + " PATH_OR_ID", Short: "Start " + string(mode) + " forwarding", Args: exactArgs(1)}
		cmd.Long = "Run in the foreground until interruption. Local and Dynamic reach destinations from the saved host; Remote reaches destinations from this computer. Remote listeners are requested, not verified: server policy controls actual exposure and release after transport loss. Dynamic is an unauthenticated SOCKS5 CONNECT proxy. No shell, automatic reconnect, or idle timeout is used."
		cmd.Flags().String("listen-port", "", "required decimal listening port (1-65535)")
		cmd.Flags().String("listen-address", "127.0.0.1", "listening IP literal or localhost")
		cmd.Flags().String("destination", "", "fixed destination HOST:PORT (Local/Remote only)")
		cmd.Flags().Bool("non-interactive", false, "never prompt, including native credential recovery")
		cmd.Flags().Bool("acknowledge-exposure", false, "acknowledge access by other machines (does not approve host trust)")
		cmd.RunE = func(cmd *cobra.Command, args []string) error {
			config, unattended, err := tunnelFlags(cmd, options, local != nil && local.Interactive())
			if err != nil {
				return err
			}
			selector, err := parseSelector(args[0])
			if err != nil {
				return usageError("invalid connection path or ID", "", err)
			}
			if connections == nil || service == nil {
				return NewError(CodeCatalog, "tunnel service is unavailable", "", nil)
			}
			resolved, err := connections.Get(cmd.Context(), selector)
			if err != nil {
				return commandError(err, args[0])
			}
			connection := resolved.Connection
			revision := connection.Revision
			request := app.TunnelRequest{Connection: app.ItemSelector{ID: connection.ID}, Expected: &revision, Config: config, NonInteractive: unattended}
			if !unattended {
				listening, reaching := "this computer", "saved host"
				if config.Mode == app.TunnelRemote {
					listening, reaching = "saved host", "this computer"
				}
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Target: %s (SSH %s)\nMode: %s\nListening on %s: %s\n", connection.Path, net.JoinHostPort(connection.Host, strconv.Itoa(int(connection.Port))), config.Mode, listening, config.Listen); err != nil {
					return tunnelOutputError(err)
				}
				if config.Mode != app.TunnelDynamic {
					if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Destination reached from %s: %s\n", reaching, config.Destination); err != nil {
						return tunnelOutputError(err)
					}
				}
				if !net.ParseIP(config.Listen.Host).IsLoopback() && !config.ExposureAcknowledged {
					warning := "Other machines may access this listener."
					if config.Mode == app.TunnelDynamic {
						warning += " They have unauthenticated SOCKS5 proxy access through the saved host."
					}
					if err := tunnelConfirm(cmd.Context(), local, cmd.ErrOrStderr(), warning+" Acknowledge exposure [yes/no] (no): "); err != nil {
						return err
					}
					request.Config.ExposureAcknowledged = true
				}
				if err := tunnelConfirm(cmd.Context(), local, cmd.ErrOrStderr(), "Start this tunnel [yes/no] (no): "); err != nil {
					return err
				}
				request.DecideTrust = func(ctx context.Context, prompt app.TrustDecisionPrompt) (app.TrustDecision, error) {
					message := fmt.Sprintf("Host key %s: %s %s\nTrust [reject/once/persist] (reject): ", prompt.Status, prompt.Host.KeyAlgorithm, prompt.Host.FingerprintSHA256)
					answer, err := tunnelAnswer(ctx, local, cmd.ErrOrStderr(), message)
					if err != nil {
						return app.TrustReject, err
					}
					switch answer {
					case "once":
						return app.TrustOnce, nil
					case "persist":
						return app.TrustPersist, nil
					default:
						return app.TrustReject, context.Canceled
					}
				}
				request.ReadSecret = func(ctx context.Context, req app.SecretRequest) ([]byte, error) {
					message := "Password: "
					if req.Kind == app.SecretPassphrase {
						message = "Private key passphrase: "
					}
					if _, err := io.WriteString(cmd.ErrOrStderr(), message); err != nil {
						return nil, tunnelOutputError(err)
					}
					return local.ReadSecret(ctx, terminal.SecretPrompt{})
				}
			}
			if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "Starting: %s tunnel %s; requested listener: %s\n", config.Mode, connection.Path, config.Listen); err != nil {
				return tunnelOutputError(err)
			}
			snapshot, err := service.Start(cmd.Context(), request)
			if err != nil {
				return tunnelCommandError(err, connection.Path, config.Listen.String())
			}
			defer service.Stop(context.WithoutCancel(cmd.Context()), snapshot.ID)
			return runForegroundTunnel(cmd.Context(), cmd.ErrOrStderr(), service, snapshot)
		}
		parent.AddCommand(cmd)
	}
	return parent
}

func tunnelFlags(cmd *cobra.Command, options *Options, interactive bool) (app.TunnelConfig, bool, error) {
	fail := func(message string) (app.TunnelConfig, bool, error) {
		return app.TunnelConfig{}, false, usageError(message, "", nil)
	}
	if options.JSON {
		return fail("--json cannot be used with tunnel")
	}
	port, _ := cmd.Flags().GetString("listen-port")
	if port == "" {
		return fail("--listen-port is required (decimal 1-65535)")
	}
	for _, c := range port {
		if c < '0' || c > '9' {
			return fail("--listen-port must be decimal 1-65535")
		}
	}
	n, err := strconv.ParseUint(port, 10, 16)
	if err != nil || n == 0 {
		return fail("--listen-port must be decimal 1-65535")
	}
	address, _ := cmd.Flags().GetString("listen-address")
	if address == "" {
		return fail("--listen-address must be an IP literal or localhost")
	}
	unattended, _ := cmd.Flags().GetBool("non-interactive")
	ack, _ := cmd.Flags().GetBool("acknowledge-exposure")
	if !unattended && !interactive {
		return fail("without an interactive terminal, explicit --non-interactive is required")
	}
	config := app.TunnelConfig{Mode: app.TunnelMode(cmd.Name()), Listen: app.TunnelEndpoint{Host: address, Port: uint16(n)}, ExposureAcknowledged: ack}
	destination, _ := cmd.Flags().GetString("destination")
	if config.Mode == app.TunnelDynamic {
		if cmd.Flags().Changed("destination") {
			return fail("Dynamic forwarding does not accept --destination")
		}
	} else {
		config.Destination, err = app.ParseTunnelEndpoint(destination)
		if err != nil {
			return fail("--destination requires HOST:PORT (IPv6: [::1]:8080)")
		}
	}
	// Validate the exact endpoints before bootstrap; interactive exposure consent
	// is collected separately after target capture, not granted by validation.
	checked := config
	if !unattended {
		checked.ExposureAcknowledged = true
	}
	checked, err = app.ValidateTunnelConfig(checked)
	if err != nil {
		return fail("invalid tunnel endpoints or missing --acknowledge-exposure; review listening IP, ports, and destination")
	}
	checked.ExposureAcknowledged = ack
	return checked, unattended, nil
}

func tunnelAnswer(ctx context.Context, local app.Terminal, output io.Writer, message string) (string, error) {
	if _, err := io.WriteString(output, message); err != nil {
		return "", tunnelOutputError(err)
	}
	value, err := local.ReadSecret(ctx, terminal.SecretPrompt{})
	defer clear(value)
	if err != nil {
		return "", NewError(CodeCanceled, "tunnel input canceled", "", err)
	}
	if err := ctx.Err(); err != nil {
		return "", NewError(CodeCanceled, "tunnel input canceled", "", err)
	}
	return strings.ToLower(strings.TrimSpace(string(value))), nil
}

func tunnelConfirm(ctx context.Context, local app.Terminal, output io.Writer, message string) error {
	answer, err := tunnelAnswer(ctx, local, output, message)
	if err != nil {
		return err
	}
	if answer != "yes" {
		return NewError(CodeCanceled, "tunnel confirmation canceled", "", context.Canceled)
	}
	return nil
}

func tunnelOutputError(err error) error {
	return NewError(CodeInternal, "tunnel status could not be written", "", err)
}

func runForegroundTunnel(ctx context.Context, output io.Writer, service *app.TunnelService, initial app.TunnelSnapshot) error {
	done := make(chan error, 1)
	waitDone := make(chan struct{})
	go func() { defer close(waitDone); done <- service.Wait(ctx, initial.ID) }()
	// Join the waiter even on output failure; Stop joins all runtime work.
	defer func() { _ = service.Stop(context.WithoutCancel(ctx), initial.ID); <-waitDone }()
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	// Starting was printed before Start: security input owns stderr until the
	// runner becomes Active or completes, so progress never races its editor.
	state := initial.State
	var diagnostic string
	var lastDiagnostic time.Time
	printSnapshot := func() error {
		s, ok := service.Get(initial.ID)
		if !ok {
			return nil
		}
		if s.State != state {
			state = s.State
			label := "Listener"
			if s.Config.Mode == app.TunnelRemote {
				label = "Requested listener"
			}
			if _, err := fmt.Fprintf(output, "%s: %s tunnel %s; %s: %s\n", s.State, s.Config.Mode, s.Connection.Path, label, s.Config.Listen); err != nil {
				return err
			}
			if s.State == app.TunnelActive {
				if s.Config.Mode == app.TunnelRemote {
					if _, err := io.WriteString(output, "Warning: Actual remote listening scope is unverified and depends on server configuration.\n"); err != nil {
						return err
					}
				}
				if s.Warning != "" {
					if _, err := fmt.Fprintf(output, "Warning: %s\n", s.Warning); err != nil {
						return err
					}
				}
			}
		}
		if s.Diagnostic != "" && s.Diagnostic != diagnostic && time.Since(lastDiagnostic) >= time.Second {
			diagnostic, lastDiagnostic = s.Diagnostic, time.Now()
			_, err := fmt.Fprintf(output, "Notice: %s\n", diagnostic)
			return err
		}
		return nil
	}
	for {
		if err := printSnapshot(); err != nil {
			return tunnelOutputError(err)
		}
		select {
		case err := <-done:
			if ctx.Err() != nil {
				return NewError(CodeCanceled, "tunnel canceled", initial.Connection.Path, ctx.Err())
			}
			if writeErr := printSnapshot(); writeErr != nil {
				return tunnelOutputError(writeErr)
			}
			if err != nil {
				if final, ok := service.Get(initial.ID); ok && final.State == app.TunnelStopped && errors.Is(err, context.Canceled) {
					return nil
				}
				if final, ok := service.Get(initial.ID); ok && final.Diagnostic == "Forwarding listener unavailable." {
					return NewError(CodeTransport, "requested tunnel listener "+initial.Config.Listen.String()+" is unavailable; check address/port availability, permissions, and server forwarding policy before retrying", initial.Connection.Path, err)
				}
				return tunnelCommandError(err, initial.Connection.Path, initial.Config.Listen.String())
			}
			return nil
		case <-ticker.C:
		}
	}
}

func tunnelCommandError(err error, target, endpoint string) error {
	var outputFailure *ManagementError
	if errors.As(err, &outputFailure) && outputFailure.code == CodeInternal {
		return NewError(CodeInternal, "tunnel status could not be written", target, err)
	}
	var failure *app.SSHStartError
	if errors.As(err, &failure) {
		diagnostic := failure.Presentation()
		return NewError(codeForStartupDiagnostic(diagnostic), fmt.Sprintf("tunnel failed (requested listener %s): %s; %s", endpoint, diagnostic.Summary, diagnostic.Recommendation), target, err)
	}
	switch app.ErrorKindOf(err) {
	case app.ErrorKindInvalid:
		return NewError(CodeUsage, "tunnel request is invalid", target, err)
	case app.ErrorKindNotFound:
		return NewError(CodeNotFound, "connection was not found", target, err)
	case app.ErrorKindConflict:
		return NewError(CodeConflict, "the connection changed; review before retrying", target, err)
	case app.ErrorKindCanceled:
		return NewError(CodeCanceled, "tunnel canceled", target, err)
	case app.ErrorKindSecurity:
		return NewError(CodeSecurity, "host trust or required authentication is unavailable", target, err)
	default:
		return NewError(CodeInternal, "tunnel application failed", target, err)
	}
}
