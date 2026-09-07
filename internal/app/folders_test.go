package app

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestFolderServiceCRUDListAndSafeErrors(t *testing.T) {
	repository := newFolderServiceRepository()
	service, err := NewFolderService(repository, &folderCredentialLifecycle{repository: repository})
	if err != nil {
		t.Fatal(err)
	}

	expectedParent := Revision(6)
	created, err := service.Create(context.Background(), CreateFolderRequest{Parent: ItemSelector{Path: "/"}, ExpectedParent: &expectedParent, ExpectedParentPath: "/", Name: "clients"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Folder.Path != "/clients" || created.Folder.Revision != 1 {
		t.Fatalf("created = %#v", created.Folder)
	}
	if repository.lastCreate.ExpectedParent == nil || *repository.lastCreate.ExpectedParent != expectedParent || repository.lastCreate.ExpectedParentPath != "/" {
		t.Fatal("expected parent revision did not cross the repository boundary")
	}
	if got, err := service.Get(context.Background(), ItemSelector{ID: created.Folder.ID}); err != nil || got.Folder.ID != created.Folder.ID {
		t.Fatalf("Get() = %#v, %v", got, err)
	}
	if listed, err := service.List(context.Background(), ListChildrenRequest{Folder: ItemSelector{Path: "/"}}); err != nil || len(listed.Folders) != 1 {
		t.Fatalf("List() = %#v, %v", listed, err)
	}
	archive := repository.addFolder(folderRootID, "archive")

	renamed, err := service.Rename(context.Background(), RenameFolderRequest{
		Folder: ItemSelector{ID: created.Folder.ID}, Name: "customers", Expected: folderRevisionRef(created.Folder.Revision),
	})
	if err != nil {
		t.Fatal(err)
	}
	moved, err := service.Move(context.Background(), MoveFolderRequest{
		Folder: ItemSelector{ID: renamed.Folder.ID}, Destination: ItemSelector{Path: "/archive"}, Expected: folderRevisionRef(renamed.Folder.Revision), ExpectedDestination: folderRevisionRef(archive.Revision), ExpectedSourcePath: renamed.Folder.Path, ExpectedDestinationPath: archive.Path,
	})
	if err != nil || moved.Folder.Path != "/archive/customers" {
		t.Fatalf("Move() = %#v, %v", moved, err)
	}
	if repository.lastMove.ExpectedDestination == nil || *repository.lastMove.ExpectedDestination != archive.Revision || repository.lastMove.ExpectedSourcePath != renamed.Folder.Path || repository.lastMove.ExpectedDestinationPath != archive.Path {
		t.Fatal("expected destination revision did not cross the repository boundary")
	}

	const canary = "sqlite detail canary"
	repository.err = errors.New(canary)
	_, err = service.Get(context.Background(), ItemSelector{ID: moved.Folder.ID})
	if err == nil || strings.Contains(err.Error(), canary) || ErrorKindOf(err) != ErrorKindInternal {
		t.Fatalf("unsafe error = %v", err)
	}
}

func TestFolderServiceDeleteScopeIsDeterministicAndCoordinatesCredentials(t *testing.T) {
	repository := newFolderServiceRepository()
	tree := repository.addFolder(folderRootID, "tree")
	branch := repository.addFolder(tree.ID, "branch")
	plain := repository.addConnection(tree.ID, "plain", "")
	rememberedB := repository.addConnection(branch.ID, "remembered-b", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	rememberedA := repository.addConnection(tree.ID, "remembered-a", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	credentials := &folderCredentialLifecycle{repository: repository}
	service, err := NewFolderService(repository, credentials)
	if err != nil {
		t.Fatal(err)
	}

	scope, err := service.DeleteScope(context.Background(), ItemSelector{ID: tree.ID})
	if err != nil {
		t.Fatal(err)
	}
	if scope.Folders != 2 || scope.Connections != 3 || scope.RememberedCredentials != 2 {
		t.Fatalf("scope = %#v", scope)
	}
	wantOrder := []NodeID{tree.ID, branch.ID, plain.ID, rememberedB.ID, rememberedA.ID}
	for index, item := range scope.Snapshot {
		if item.ID != wantOrder[index] {
			t.Fatalf("snapshot order = %#v, want %#v", scope.Snapshot, wantOrder)
		}
	}

	deleted, err := service.Delete(context.Background(), scope.Request(true))
	if err != nil {
		t.Fatal(err)
	}
	if deleted.FoldersDeleted != 2 || deleted.ConnectionsDeleted != 3 {
		t.Fatalf("deleted = %#v", deleted)
	}
	if len(credentials.deleted) != 2 || credentials.deleted[0] != rememberedB.ID || credentials.deleted[1] != rememberedA.ID {
		t.Fatalf("credential deletion order = %#v", credentials.deleted)
	}
	if len(repository.lastDelete.Snapshot) != 3 {
		t.Fatalf("final repository snapshot = %#v", repository.lastDelete.Snapshot)
	}
}

func TestFolderServiceRejectsStaleDeleteScopeBeforeCredentialChanges(t *testing.T) {
	repository := newFolderServiceRepository()
	tree := repository.addFolder(folderRootID, "tree")
	repository.addConnection(tree.ID, "remembered", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	credentials := &folderCredentialLifecycle{repository: repository}
	service, err := NewFolderService(repository, credentials)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := service.DeleteScope(context.Background(), ItemSelector{ID: tree.ID})
	if err != nil {
		t.Fatal(err)
	}
	repository.addFolder(tree.ID, "late")

	_, err = service.Delete(context.Background(), scope.Request(true))
	if !errors.Is(err, ErrConflict) || len(credentials.deleted) != 0 || repository.deleteCalls != 0 {
		t.Fatalf("stale delete error/calls = %v / %v / %d", err, credentials.deleted, repository.deleteCalls)
	}
}

func TestFolderServiceRejectsRootDeleteScopeBeforeCredentialChanges(t *testing.T) {
	repository := newFolderServiceRepository()
	repository.addConnection(folderRootID, "remembered", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	credentials := &folderCredentialLifecycle{repository: repository}
	service, err := NewFolderService(repository, credentials)
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.DeleteScope(context.Background(), ItemSelector{Path: "/"})
	if !errors.Is(err, ErrConflict) || len(credentials.deleted) != 0 || repository.deleteCalls != 0 {
		t.Fatalf("root scope error/calls = %v / %v / %d", err, credentials.deleted, repository.deleteCalls)
	}
}

func TestFolderServiceCredentialFailureLeavesTrackedSagaBoundaryRecoverable(t *testing.T) {
	repository := newFolderServiceRepository()
	tree := repository.addFolder(folderRootID, "tree")
	first := repository.addConnection(tree.ID, "first", "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	second := repository.addConnection(tree.ID, "second", "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb")
	credentials := &folderCredentialLifecycle{repository: repository, failID: second.ID, err: errors.New("credential store unavailable")}
	service, err := NewFolderService(repository, credentials)
	if err != nil {
		t.Fatal(err)
	}
	scope, err := service.DeleteScope(context.Background(), ItemSelector{ID: tree.ID})
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.Delete(context.Background(), scope.Request(true))
	if err == nil || repository.deleteCalls != 0 {
		t.Fatalf("Delete() error/calls = %v / %d", err, repository.deleteCalls)
	}
	if _, exists := repository.nodes[first.ID]; exists {
		t.Fatal("first saga-completed connection still exists")
	}
	if _, exists := repository.nodes[second.ID]; !exists {
		t.Fatal("failed saga connection was removed without recoverable ownership")
	}
}

const folderRootID = NodeID("00000000000000000000000000000001")

type folderServiceRepository struct {
	nodes       map[NodeID]Node
	connections map[NodeID]Connection
	next        uint64
	err         error
	lastCreate  CreateFolderRequest
	lastMove    MoveFolderRequest
	lastDelete  DeleteFolderRequest
	deleteCalls int
}

func newFolderServiceRepository() *folderServiceRepository {
	return &folderServiceRepository{
		nodes:       map[NodeID]Node{folderRootID: {ID: folderRootID, Kind: NodeKindFolder, Name: ".", Path: "/", Revision: 1}},
		connections: make(map[NodeID]Connection), next: 1,
	}
}

func (r *folderServiceRepository) id() NodeID {
	r.next++
	return NodeID(strings.Repeat("0", 31) + string("0123456789abcdef"[r.next%16]))
}

func (r *folderServiceRepository) addFolder(parent NodeID, name string) Folder {
	id := r.id()
	path := r.nodes[parent].Path
	if path != "/" {
		path += "/"
	}
	node := Node{ID: id, ParentID: parent, Kind: NodeKindFolder, Name: name, Path: path + name, Revision: 1}
	r.nodes[id] = node
	return Folder{Node: node}
}

func (r *folderServiceRepository) addConnection(parent NodeID, name, ref string) Connection {
	id := r.id()
	path := r.nodes[parent].Path
	if path != "/" {
		path += "/"
	}
	connection := Connection{Node: Node{ID: id, ParentID: parent, Kind: NodeKindConnection, Name: name, Path: path + name, Revision: 1}, Host: name + ".example", Port: 22, AuthMethod: AuthMethodPassword, CredentialRef: ref}
	r.nodes[id] = connection.Node
	r.connections[id] = connection
	return connection
}

func (r *folderServiceRepository) CreateFolder(_ context.Context, request CreateFolderRequest) (FolderResult, error) {
	if r.err != nil {
		return FolderResult{}, r.err
	}
	r.lastCreate = request
	parent := folderRootID
	if request.Parent.ID != "" {
		parent = request.Parent.ID
	}
	folder := r.addFolder(parent, request.Name)
	return FolderResult{Folder: folder, CatalogRevision: 2}, nil
}

func (r *folderServiceRepository) GetFolder(_ context.Context, selector ItemSelector) (FolderResult, error) {
	if r.err != nil {
		return FolderResult{}, r.err
	}
	for _, node := range r.nodes {
		if node.Kind == NodeKindFolder && (selector.ID == node.ID || selector.Path == node.Path) {
			return FolderResult{Folder: Folder{Node: node}, CatalogRevision: 2}, nil
		}
	}
	return FolderResult{}, ErrNotFound
}

func (r *folderServiceRepository) ListChildren(_ context.Context, request ListChildrenRequest) (ListChildrenResult, error) {
	folder, err := r.GetFolder(context.Background(), request.Folder)
	if err != nil {
		return ListChildrenResult{}, err
	}
	result := ListChildrenResult{CatalogRevision: 2}
	for id, node := range r.nodes {
		if node.ParentID != folder.Folder.ID {
			continue
		}
		if node.Kind == NodeKindFolder {
			result.Folders = append(result.Folders, Folder{Node: node})
		} else {
			result.Connections = append(result.Connections, r.connections[id])
		}
	}
	return result, nil
}

func (r *folderServiceRepository) RenameFolder(_ context.Context, request RenameFolderRequest) (FolderResult, error) {
	folder, err := r.GetFolder(context.Background(), request.Folder)
	if err != nil {
		return FolderResult{}, err
	}
	node := folder.Folder.Node
	node.Name = request.Name
	node.Revision++
	r.nodes[node.ID] = node
	return FolderResult{Folder: Folder{Node: node}, CatalogRevision: 3}, nil
}

func (r *folderServiceRepository) MoveFolder(_ context.Context, request MoveFolderRequest) (FolderResult, error) {
	r.lastMove = request
	folder, err := r.GetFolder(context.Background(), request.Folder)
	if err != nil {
		return FolderResult{}, err
	}
	destination, err := r.GetFolder(context.Background(), request.Destination)
	if err != nil {
		return FolderResult{}, err
	}
	node := folder.Folder.Node
	node.ParentID = destination.Folder.ID
	node.Path = destination.Folder.Path + "/" + node.Name
	node.Revision++
	r.nodes[node.ID] = node
	return FolderResult{Folder: Folder{Node: node}, CatalogRevision: 4}, nil
}

func (r *folderServiceRepository) DeleteFolder(_ context.Context, request DeleteFolderRequest) (DeleteFolderResult, error) {
	r.deleteCalls++
	r.lastDelete = request
	folders, connections := 0, 0
	for _, item := range request.Snapshot {
		node, exists := r.nodes[item.ID]
		if !exists || node.Revision != item.Revision {
			return DeleteFolderResult{}, ErrConflict
		}
		if node.Kind == NodeKindFolder {
			folders++
		} else {
			connections++
		}
	}
	for _, item := range request.Snapshot {
		delete(r.nodes, item.ID)
		delete(r.connections, item.ID)
	}
	return DeleteFolderResult{FoldersDeleted: folders, ConnectionsDeleted: connections, CatalogRevision: 5}, nil
}

type folderCredentialLifecycle struct {
	repository *folderServiceRepository
	deleted    []NodeID
	failID     NodeID
	err        error
}

func (*folderCredentialLifecycle) Save(context.Context, Connection, []byte) error    { return nil }
func (*folderCredentialLifecycle) Replace(context.Context, Connection, []byte) error { return nil }
func (*folderCredentialLifecycle) Remove(context.Context, Connection, AuthMethod, string) error {
	return nil
}
func (f *folderCredentialLifecycle) DeleteConnection(_ context.Context, connection Connection) error {
	if connection.ID == f.failID {
		return f.err
	}
	f.deleted = append(f.deleted, connection.ID)
	delete(f.repository.nodes, connection.ID)
	delete(f.repository.connections, connection.ID)
	return nil
}
func (*folderCredentialLifecycle) Recover(context.Context) error { return nil }

func folderRevisionRef(revision Revision) *Revision { return &revision }
