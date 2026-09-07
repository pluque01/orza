package tui

import (
	"context"
	"errors"

	"github.com/pluque01/orza/internal/app"
)

// ConnectionService is the application boundary used by the TUI. The concrete
// app.ConnectionService satisfies it without a presentation-layer adapter.
type ConnectionService interface {
	Create(context.Context, app.CreateConnectionRequest) (app.ConnectionResult, error)
	Get(context.Context, app.ItemSelector) (app.ConnectionResult, error)
	List(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error)
	Update(context.Context, app.UpdateConnectionRequest) (app.ConnectionResult, error)
	Move(context.Context, app.MoveConnectionRequest) (app.ConnectionResult, error)
	DeleteScope(context.Context, app.ItemSelector) (app.ConnectionDeleteScope, error)
	Delete(context.Context, app.DeleteConnectionRequest) (app.DeleteConnectionResult, error)
}

// FolderService is the shared hierarchy use-case boundary used by the TUI.
type FolderService interface {
	Create(context.Context, app.CreateFolderRequest) (app.FolderResult, error)
	Get(context.Context, app.ItemSelector) (app.FolderResult, error)
	List(context.Context, app.ListChildrenRequest) (app.ListChildrenResult, error)
	Rename(context.Context, app.RenameFolderRequest) (app.FolderResult, error)
	Move(context.Context, app.MoveFolderRequest) (app.FolderResult, error)
	DeleteScope(context.Context, app.ItemSelector) (app.FolderDeleteScope, error)
	Delete(context.Context, app.DeleteFolderRequest) (app.DeleteFolderResult, error)
}

// ConnectService is the application session boundary used by the TUI.
type ConnectService interface {
	Connect(context.Context, app.ConnectRequest) (app.ConnectResult, error)
}

var errUnavailable = errors.New("TUI service is unavailable")

// ConnectionFuncs is a compact deterministic adapter for model tests.
type ConnectionFuncs struct {
	CreateFunc      func(context.Context, app.CreateConnectionRequest) (app.ConnectionResult, error)
	GetFunc         func(context.Context, app.ItemSelector) (app.ConnectionResult, error)
	ListFunc        func(context.Context, app.ListConnectionsRequest) (app.ListConnectionsResult, error)
	UpdateFunc      func(context.Context, app.UpdateConnectionRequest) (app.ConnectionResult, error)
	MoveFunc        func(context.Context, app.MoveConnectionRequest) (app.ConnectionResult, error)
	DeleteScopeFunc func(context.Context, app.ItemSelector) (app.ConnectionDeleteScope, error)
	DeleteFunc      func(context.Context, app.DeleteConnectionRequest) (app.DeleteConnectionResult, error)
}

func (f ConnectionFuncs) Create(ctx context.Context, request app.CreateConnectionRequest) (app.ConnectionResult, error) {
	if f.CreateFunc == nil {
		return app.ConnectionResult{}, errUnavailable
	}
	return f.CreateFunc(ctx, request)
}

func (f ConnectionFuncs) Get(ctx context.Context, selector app.ItemSelector) (app.ConnectionResult, error) {
	if f.GetFunc == nil {
		return app.ConnectionResult{}, errUnavailable
	}
	return f.GetFunc(ctx, selector)
}

func (f ConnectionFuncs) List(ctx context.Context, request app.ListConnectionsRequest) (app.ListConnectionsResult, error) {
	if f.ListFunc == nil {
		return app.ListConnectionsResult{}, errUnavailable
	}
	return f.ListFunc(ctx, request)
}

func (f ConnectionFuncs) Update(ctx context.Context, request app.UpdateConnectionRequest) (app.ConnectionResult, error) {
	if f.UpdateFunc == nil {
		return app.ConnectionResult{}, errUnavailable
	}
	return f.UpdateFunc(ctx, request)
}

func (f ConnectionFuncs) Move(ctx context.Context, request app.MoveConnectionRequest) (app.ConnectionResult, error) {
	if f.MoveFunc == nil {
		return app.ConnectionResult{}, errUnavailable
	}
	return f.MoveFunc(ctx, request)
}

func (f ConnectionFuncs) DeleteScope(ctx context.Context, selector app.ItemSelector) (app.ConnectionDeleteScope, error) {
	if f.DeleteScopeFunc == nil {
		return app.ConnectionDeleteScope{}, errUnavailable
	}
	return f.DeleteScopeFunc(ctx, selector)
}

func (f ConnectionFuncs) Delete(ctx context.Context, request app.DeleteConnectionRequest) (app.DeleteConnectionResult, error) {
	if f.DeleteFunc == nil {
		return app.DeleteConnectionResult{}, errUnavailable
	}
	return f.DeleteFunc(ctx, request)
}

// FolderFuncs is a deterministic adapter for hierarchy model tests.
type FolderFuncs struct {
	CreateFunc      func(context.Context, app.CreateFolderRequest) (app.FolderResult, error)
	GetFunc         func(context.Context, app.ItemSelector) (app.FolderResult, error)
	ListFunc        func(context.Context, app.ListChildrenRequest) (app.ListChildrenResult, error)
	RenameFunc      func(context.Context, app.RenameFolderRequest) (app.FolderResult, error)
	MoveFunc        func(context.Context, app.MoveFolderRequest) (app.FolderResult, error)
	DeleteScopeFunc func(context.Context, app.ItemSelector) (app.FolderDeleteScope, error)
	DeleteFunc      func(context.Context, app.DeleteFolderRequest) (app.DeleteFolderResult, error)
}

func (f FolderFuncs) Create(ctx context.Context, request app.CreateFolderRequest) (app.FolderResult, error) {
	if f.CreateFunc == nil {
		return app.FolderResult{}, errUnavailable
	}
	return f.CreateFunc(ctx, request)
}

func (f FolderFuncs) Get(ctx context.Context, selector app.ItemSelector) (app.FolderResult, error) {
	if f.GetFunc == nil {
		return app.FolderResult{}, errUnavailable
	}
	return f.GetFunc(ctx, selector)
}

func (f FolderFuncs) List(ctx context.Context, request app.ListChildrenRequest) (app.ListChildrenResult, error) {
	if f.ListFunc == nil {
		return app.ListChildrenResult{}, errUnavailable
	}
	return f.ListFunc(ctx, request)
}

func (f FolderFuncs) Rename(ctx context.Context, request app.RenameFolderRequest) (app.FolderResult, error) {
	if f.RenameFunc == nil {
		return app.FolderResult{}, errUnavailable
	}
	return f.RenameFunc(ctx, request)
}

func (f FolderFuncs) Move(ctx context.Context, request app.MoveFolderRequest) (app.FolderResult, error) {
	if f.MoveFunc == nil {
		return app.FolderResult{}, errUnavailable
	}
	return f.MoveFunc(ctx, request)
}

func (f FolderFuncs) DeleteScope(ctx context.Context, selector app.ItemSelector) (app.FolderDeleteScope, error) {
	if f.DeleteScopeFunc == nil {
		return app.FolderDeleteScope{}, errUnavailable
	}
	return f.DeleteScopeFunc(ctx, selector)
}

func (f FolderFuncs) Delete(ctx context.Context, request app.DeleteFolderRequest) (app.DeleteFolderResult, error) {
	if f.DeleteFunc == nil {
		return app.DeleteFolderResult{}, errUnavailable
	}
	return f.DeleteFunc(ctx, request)
}

type ConnectFunc func(context.Context, app.ConnectRequest) (app.ConnectResult, error)

func (f ConnectFunc) Connect(ctx context.Context, request app.ConnectRequest) (app.ConnectResult, error) {
	if f == nil {
		return app.ConnectResult{}, errUnavailable
	}
	return f(ctx, request)
}
