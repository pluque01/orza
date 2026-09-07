package main

import (
	"context"
	"errors"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/catalogrepo"
	"github.com/pluque01/orza/internal/cli"
	"github.com/pluque01/orza/internal/credential"
	"github.com/pluque01/orza/internal/hostkey"
	"github.com/pluque01/orza/internal/sshclient"
	"github.com/pluque01/orza/internal/terminal"
)

var version = "dev"

var (
	notifySignals = signal.Notify
	stopSignals   = signal.Stop
)

var bootstrap = func(ctx context.Context) (*app.Dependencies, error) {
	return app.Bootstrap(ctx, app.BootstrapOptions{
		Configure: func(ctx context.Context, dependencies *app.Dependencies) error {
			repository := catalogrepo.NewRepository(dependencies.Catalog)
			credentialStore := credential.NewStore()
			catalogID, err := dependencies.Catalog.CatalogID(ctx)
			if err != nil {
				return err
			}
			scope := credential.Scope(catalogID)
			saga, err := app.NewCredentialSaga(app.NewCatalogCredentialOperationRepository(dependencies.Catalog), credentialStore, scope, nil)
			if err != nil {
				return err
			}
			connections, err := app.NewConnectionService(repository, saga)
			if err != nil {
				return err
			}
			folders, err := app.NewFolderService(repository, saga)
			if err != nil {
				return err
			}
			catalogTrustedHosts := catalog.NewTrustedHostRepository(dependencies.Catalog)
			trustedHosts := hostkey.NewCatalogTrustedHostAdapter(catalogTrustedHosts)
			hostTrust, err := hostkey.NewCatalogPolicy(catalogTrustedHosts)
			if err != nil {
				return err
			}
			localTerminal := terminal.New(os.Stdin, os.Stdout)
			runner := sshclient.New(sshclient.Options{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
			connectService, err := app.NewConnectService(app.ConnectOptions{
				Connections: repository, Credentials: saga, HostTrust: hostTrust,
				TrustedHosts: trustedHosts, Store: credentialStore, Scope: scope,
				Runner: runner, Terminal: localTerminal,
			})
			if err != nil {
				return err
			}
			dependencies.Connections = connections
			dependencies.Folders = folders
			dependencies.Connect = connectService
			dependencies.Credentials = saga
			dependencies.Terminal = localTerminal
			return nil
		},
	})
}

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func realMain(args []string, stdin io.Reader, stdout, stderr io.Writer) (exitCode int) {
	ctx, signals := ownRootSignals(context.Background())
	defer func() {
		if signalCode := signals.Stop(); signalCode != 0 {
			exitCode = signalCode
		}
	}()
	jsonOutput := requestsJSON(args)
	if bootstrapFreeInvocation(args) {
		options := &cli.Options{}
		root := cli.NewRoot(cli.RootConfig{
			Version: version,
			Stdin:   stdin,
			Stdout:  stdout,
			Stderr:  stderr,
			Options: options,
		})
		root.SetArgs(args)
		if err := root.ExecuteContext(ctx); err != nil {
			if signalCode := signals.ExitCode(); signalCode != 0 {
				return signalCode
			}
			_ = cli.WriteError(stderr, options.JSON || jsonOutput, err)
			return cli.ExitCode(err)
		}
		return cli.ExitSuccess
	}
	dependencies, err := bootstrap(ctx)
	if err != nil {
		if signalCode := signals.ExitCode(); signalCode != 0 {
			return signalCode
		}
		err = bootstrapError(err)
		_ = cli.WriteError(stderr, jsonOutput, err)
		return cli.ExitCode(err)
	}
	defer func() {
		if err := dependencies.Close(); err != nil && exitCode == cli.ExitSuccess {
			err = cli.NewError(cli.CodeCatalog, "catalog could not be closed safely", "", err)
			_ = cli.WriteError(stderr, jsonOutput, err)
			exitCode = cli.ExitCode(err)
		}
	}()

	options := &cli.Options{}
	root := cli.NewRoot(cli.RootConfig{
		Version:      version,
		Dependencies: dependencies,
		Stdin:        stdin,
		Stdout:       stdout,
		Stderr:       stderr,
		Options:      options,
	})
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		if signalCode := signals.ExitCode(); signalCode != 0 {
			return signalCode
		}
		_ = cli.WriteError(stderr, options.JSON || jsonOutput, err)
		return cli.ExitCode(err)
	}
	jsonOutput = options.JSON || jsonOutput
	return cli.ExitSuccess
}

func bootstrapError(err error) error {
	if errors.Is(err, credential.ErrUnavailable) {
		return cli.NewError(cli.CodeSecurity, "secure credential store is unavailable; start or unlock the operating-system credential service and retry", "", err)
	}
	return cli.NewError(cli.CodeCatalog, "catalog is unavailable; verify its location and permissions", "", err)
}

type rootSignals struct {
	cancel   context.CancelFunc
	signals  chan os.Signal
	done     chan struct{}
	stopOnce sync.Once
	mu       sync.Mutex
	received os.Signal
}

func ownRootSignals(parent context.Context) (context.Context, *rootSignals) {
	ctx, cancel := context.WithCancel(parent)
	owner := &rootSignals{
		cancel:  cancel,
		signals: make(chan os.Signal, 1),
		done:    make(chan struct{}),
	}
	notifySignals(owner.signals, os.Interrupt, syscall.Signal(15))
	go func() {
		defer close(owner.done)
		select {
		case received := <-owner.signals:
			owner.mu.Lock()
			owner.received = received
			owner.mu.Unlock()
			cancel()
		case <-ctx.Done():
		}
	}()
	return ctx, owner
}

func (s *rootSignals) ExitCode() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return signalExitCode(s.received)
}

func (s *rootSignals) Stop() int {
	s.stopOnce.Do(func() {
		stopSignals(s.signals)
		s.cancel()
		<-s.done
	})
	return s.ExitCode()
}

func signalExitCode(received os.Signal) int {
	switch received {
	case os.Interrupt:
		return 130
	case syscall.Signal(15):
		return 143
	default:
		return 0
	}
}

func requestsJSON(args []string) bool {
	enabled := false
	for _, arg := range args {
		if arg == "--" {
			break
		}
		if arg == "--json" {
			enabled = true
		}
		if strings.HasPrefix(arg, "--json=") {
			value, err := strconv.ParseBool(strings.TrimPrefix(arg, "--json="))
			if err == nil {
				enabled = value
			}
		}
	}
	return enabled
}

func bootstrapFreeInvocation(args []string) bool {
	firstArgument := ""
	for _, arg := range args {
		if arg == "--" {
			break
		}
		switch arg {
		case "-h", "--help", "--version":
			return true
		}
		for _, flag := range []string{"-h=", "--help=", "--version="} {
			if strings.HasPrefix(arg, flag) {
				enabled, err := strconv.ParseBool(strings.TrimPrefix(arg, flag))
				if err == nil && enabled {
					return true
				}
			}
		}
		if firstArgument == "" && !strings.HasPrefix(arg, "-") {
			firstArgument = arg
		}
	}
	return firstArgument == "help" || firstArgument == "version"
}
