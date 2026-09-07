package app

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/pluque01/orza/internal/catalog"
)

// BootstrapOptions exposes startup boundaries for deterministic tests and for
// later platform adapter registration. Nil functions use production defaults.
type BootstrapOptions struct {
	ResolveCatalogPath        func() (string, error)
	OpenCatalog               func(string) (*catalog.Store, error)
	Configure                 func(context.Context, *Dependencies) error
	RecoverPendingCredentials func(context.Context, *Dependencies) error
}

// Dependencies owns process-lifetime application resources.
type Dependencies struct {
	Catalog     *catalog.Store
	Connections *ConnectionService
	Folders     *FolderService
	Connect     *ConnectService
	Credentials CredentialLifecycle
	Terminal    Terminal

	closeOnce sync.Once
	closeErr  error
}

// Bootstrap opens and validates the default catalog before running recovery.
// No dependencies escape when any startup stage fails.
func Bootstrap(ctx context.Context, options BootstrapOptions) (*Dependencies, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	resolveCatalogPath := options.ResolveCatalogPath
	if resolveCatalogPath == nil {
		resolveCatalogPath = catalog.DefaultPath
	}
	openCatalog := options.OpenCatalog
	if openCatalog == nil {
		openCatalog = catalog.Open
	}

	path, err := resolveCatalogPath()
	if err != nil {
		return nil, fmt.Errorf("resolve catalog location: %w", err)
	}
	store, err := openCatalog(path)
	if err != nil {
		return nil, fmt.Errorf("open catalog: %w", err)
	}
	if store == nil {
		return nil, errors.New("open catalog: no catalog returned")
	}
	dependencies := &Dependencies{Catalog: store}
	fail := func(startupErr error) (*Dependencies, error) {
		if closeErr := dependencies.Close(); closeErr != nil {
			return nil, errors.Join(startupErr, fmt.Errorf("close catalog: %w", closeErr))
		}
		return nil, startupErr
	}

	if err := store.CheckIntegrity(ctx); err != nil {
		return fail(fmt.Errorf("check catalog integrity: %w", err))
	}
	if options.Configure != nil {
		if err := options.Configure(ctx, dependencies); err != nil {
			return fail(fmt.Errorf("configure application dependencies: %w", err))
		}
	}
	if options.RecoverPendingCredentials != nil {
		if err := options.RecoverPendingCredentials(ctx, dependencies); err != nil {
			return fail(fmt.Errorf("recover pending credential operations: %w", err))
		}
	} else if dependencies.Credentials != nil {
		if err := dependencies.Credentials.Recover(ctx); err != nil {
			return fail(fmt.Errorf("recover pending credential operations: %w", err))
		}
	}
	return dependencies, nil
}

// Close releases all bootstrap resources once. It is safe on nil and repeated calls.
func (d *Dependencies) Close() error {
	if d == nil {
		return nil
	}
	d.closeOnce.Do(func() {
		if d.Catalog != nil {
			d.closeErr = d.Catalog.Close()
		}
	})
	return d.closeErr
}
