package app

import (
	"context"
	"errors"
	"sort"
)

const maxFolderScopeDepth = 100

type folderScopeState struct {
	scope       FolderDeleteScope
	connections []Connection
}

func (s *FolderService) DeleteScope(ctx context.Context, selector ItemSelector) (FolderDeleteScope, error) {
	target := selectorTarget(selector)
	if err := contextError(ctx); err != nil {
		return FolderDeleteScope{}, safeFolderUseCaseError("inspect folder deletion", target, err)
	}
	if err := validateSelector(selector); err != nil {
		return FolderDeleteScope{}, safeFolderUseCaseError("inspect folder deletion", target, errors.Join(ErrInvalidRequest, err))
	}
	state, err := s.buildDeleteScope(ctx, selector)
	if err != nil {
		return FolderDeleteScope{}, safeFolderUseCaseError("inspect folder deletion", target, err)
	}
	if state.scope.Path == "/" {
		return FolderDeleteScope{}, safeFolderUseCaseError("inspect folder deletion", target, ErrConflict)
	}
	return state.scope, nil
}

func (s *FolderService) Delete(ctx context.Context, request DeleteFolderRequest) (DeleteFolderResult, error) {
	target := selectorTarget(request.Folder)
	if err := contextError(ctx); err != nil {
		return DeleteFolderResult{}, safeFolderUseCaseError("delete folder", target, err)
	}
	if err := validateSelector(request.Folder); err != nil || request.Expected != nil && *request.Expected == 0 {
		return DeleteFolderResult{}, safeFolderUseCaseError("delete folder", target, errors.Join(ErrInvalidRequest, err))
	}
	if !request.Recursive {
		result, err := s.repository.DeleteFolder(ctx, request)
		if err != nil {
			return DeleteFolderResult{}, safeFolderUseCaseError("delete folder", target, err)
		}
		return result, nil
	}
	if request.Expected == nil || !validSnapshot(request.Snapshot) {
		return DeleteFolderResult{}, safeFolderUseCaseError("delete folder", target, ErrInvalidRequest)
	}

	// Preflight before crossing into the credential store. This rejects a stale
	// confirmation without causing any partial external side effects.
	current, err := s.buildDeleteScope(ctx, request.Folder)
	if err != nil {
		return DeleteFolderResult{}, safeFolderUseCaseError("inspect folder deletion", target, err)
	}
	if current.scope.Path == "/" || current.scope.Revision != *request.Expected || !sameSnapshot(current.scope.Snapshot, request.Snapshot) {
		return DeleteFolderResult{}, safeFolderUseCaseError("delete folder", target, ErrConflict)
	}

	credentialConnections := make([]Connection, 0)
	for _, connection := range current.connections {
		if connection.CredentialRef != "" {
			credentialConnections = append(credentialConnections, connection)
		}
	}
	sort.Slice(credentialConnections, func(i, j int) bool { return credentialConnections[i].ID < credentialConnections[j].ID })

	removed := make(map[NodeID]struct{}, len(credentialConnections))
	for _, connection := range credentialConnections {
		// Each saga owns its credential and catalog node together. Multiple native
		// credential operations cannot be one SQLite transaction: a later failure
		// may leave earlier connections deleted, while the failing saga either
		// compensates or remains durable for startup recovery. No secret is left
		// untracked; callers must reload a new scope before retrying the subtree.
		if err := s.credentials.DeleteConnection(ctx, connection); err != nil {
			return DeleteFolderResult{}, safeFolderUseCaseError("delete folder credentials", target, err)
		}
		removed[connection.ID] = struct{}{}
	}

	finalRequest := request
	finalRequest.Snapshot = make([]NodeRevision, 0, len(request.Snapshot)-len(removed))
	for _, item := range request.Snapshot {
		if _, wasRemoved := removed[item.ID]; !wasRemoved {
			finalRequest.Snapshot = append(finalRequest.Snapshot, item)
		}
	}
	result, err := s.repository.DeleteFolder(ctx, finalRequest)
	if err != nil {
		return DeleteFolderResult{}, safeFolderUseCaseError("delete folder", target, err)
	}
	result.ConnectionsDeleted += len(removed)
	return result, nil
}

func (s *FolderService) buildDeleteScope(ctx context.Context, selector ItemSelector) (folderScopeState, error) {
	root, err := s.repository.GetFolder(ctx, selector)
	if err != nil {
		return folderScopeState{}, err
	}
	type pendingFolder struct {
		folder Folder
		depth  int
	}
	pending := []pendingFolder{{folder: root.Folder}}
	seen := make(map[NodeID]struct{})
	nodes := make([]NodeRevision, 0)
	connections := make([]Connection, 0)
	folderCount := 0
	remembered := 0

	for len(pending) != 0 {
		current := pending[0]
		pending = pending[1:]
		if current.depth > maxFolderScopeDepth {
			return folderScopeState{}, ErrConflict
		}
		if _, duplicate := seen[current.folder.ID]; duplicate {
			return folderScopeState{}, ErrConflict
		}
		seen[current.folder.ID] = struct{}{}
		nodes = append(nodes, NodeRevision{ID: current.folder.ID, Revision: current.folder.Revision})
		folderCount++

		children, err := s.repository.ListChildren(ctx, ListChildrenRequest{Folder: ItemSelector{ID: current.folder.ID}})
		if err != nil {
			return folderScopeState{}, err
		}
		if children.CatalogRevision != root.CatalogRevision {
			return folderScopeState{}, ErrConflict
		}
		for _, connection := range children.Connections {
			if _, duplicate := seen[connection.ID]; duplicate {
				return folderScopeState{}, ErrConflict
			}
			seen[connection.ID] = struct{}{}
			nodes = append(nodes, NodeRevision{ID: connection.ID, Revision: connection.Revision})
			connections = append(connections, connection)
			if connection.CredentialRef != "" {
				remembered++
			}
		}
		for _, folder := range children.Folders {
			pending = append(pending, pendingFolder{folder: folder, depth: current.depth + 1})
		}
	}

	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	sort.Slice(connections, func(i, j int) bool { return connections[i].ID < connections[j].ID })
	return folderScopeState{
		scope: FolderDeleteScope{
			ID: root.Folder.ID, Path: root.Folder.Path, Name: root.Folder.Name, Revision: root.Folder.Revision,
			Folders: folderCount, Connections: len(connections), RememberedCredentials: remembered,
			Snapshot: nodes,
		},
		connections: connections,
	}, nil
}

func validSnapshot(snapshot []NodeRevision) bool {
	if len(snapshot) == 0 {
		return false
	}
	seen := make(map[NodeID]struct{}, len(snapshot))
	for _, item := range snapshot {
		if item.ID == "" || item.Revision == 0 {
			return false
		}
		if _, duplicate := seen[item.ID]; duplicate {
			return false
		}
		seen[item.ID] = struct{}{}
	}
	return true
}

func sameSnapshot(left, right []NodeRevision) bool {
	if len(left) != len(right) {
		return false
	}
	want := make(map[NodeID]Revision, len(left))
	for _, item := range left {
		want[item.ID] = item.Revision
	}
	for _, item := range right {
		if want[item.ID] != item.Revision {
			return false
		}
	}
	return true
}
