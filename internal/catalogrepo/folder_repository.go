package catalogrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/domain"
)

const maxHierarchyDepth = 100

var (
	ErrRootImmutable    = errors.New("catalog root is immutable")
	ErrFolderNotEmpty   = errors.New("folder is not empty")
	ErrFolderCycle      = errors.New("folder move would create a cycle")
	ErrHierarchyTooDeep = errors.New("catalog hierarchy is too deep")
)

var _ app.FolderRepository = (*Repository)(nil)

func (repository *Repository) CreateFolder(ctx context.Context, request app.CreateFolderRequest) (result app.FolderResult, err error) {
	name, err := domain.NewName(request.Name)
	if err != nil {
		return result, err
	}
	id, err := domain.NewID()
	if err != nil {
		return result, fmt.Errorf("generate folder ID: %w", err)
	}

	err = repository.withImmediate(ctx, "create folder", func(connection *sql.Conn) error {
		parent, err := resolveSelector(ctx, connection, request.Parent)
		if err != nil {
			return err
		}
		if parent.kind != app.NodeKindFolder {
			return ErrWrongKind
		}
		parentFolder, err := folderProjection(ctx, connection, parent.id)
		if err != nil {
			return err
		}
		if err := checkExpectedRevision(parentFolder.Revision, request.ExpectedParent); err != nil {
			return err
		}
		if err := checkExpectedPath(parentFolder.Path, request.ExpectedParentPath); err != nil {
			return err
		}
		if err := ensureNameAvailable(ctx, connection, parent.id, name.String(), ""); err != nil {
			return err
		}
		now := formatCatalogTime(time.Now())
		if _, err := connection.ExecContext(ctx, `
			INSERT INTO nodes(id, parent_id, kind, name, revision, created_at, updated_at)
			VALUES(?, ?, 'folder', ?, 1, ?, ?)
		`, id.String(), parent.id, name.String(), now, now); err != nil {
			return fmt.Errorf("insert folder: %w", err)
		}
		result.CatalogRevision, err = incrementCatalogRevision(ctx, connection)
		if err != nil {
			return err
		}
		result.Folder, err = folderProjection(ctx, connection, id.String())
		return err
	})
	return result, err
}

func (repository *Repository) GetFolder(ctx context.Context, selector app.ItemSelector) (result app.FolderResult, err error) {
	err = repository.withRead(ctx, func(transaction *sql.Tx) error {
		item, err := resolveSelector(ctx, transaction, selector)
		if err != nil {
			return err
		}
		if item.kind != app.NodeKindFolder {
			return ErrWrongKind
		}
		result.Folder, err = folderProjection(ctx, transaction, item.id)
		if err != nil {
			return err
		}
		result.CatalogRevision, err = readCatalogRevision(ctx, transaction)
		return err
	})
	return result, err
}

func (repository *Repository) ListChildren(ctx context.Context, request app.ListChildrenRequest) (result app.ListChildrenResult, err error) {
	err = repository.withRead(ctx, func(transaction *sql.Tx) error {
		folder, err := resolveSelector(ctx, transaction, request.Folder)
		if err != nil {
			return err
		}
		if folder.kind != app.NodeKindFolder {
			return ErrWrongKind
		}

		rows, err := transaction.QueryContext(ctx, `
			SELECT id, kind FROM nodes
			WHERE parent_id = ?
			ORDER BY kind = 'connection', name COLLATE BINARY, id
		`, folder.id)
		if err != nil {
			return fmt.Errorf("list folder children: %w", err)
		}
		var children []catalogItem
		for rows.Next() {
			var child catalogItem
			if err := rows.Scan(&child.id, &child.kind); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan folder child: %w", err)
			}
			children = append(children, child)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close folder children: %w", err)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("list folder children: %w", err)
		}

		result.Folders = make([]app.Folder, 0)
		result.Connections = make([]app.Connection, 0)
		for _, child := range children {
			switch child.kind {
			case app.NodeKindFolder:
				projection, err := folderProjection(ctx, transaction, child.id)
				if err != nil {
					return err
				}
				result.Folders = append(result.Folders, projection)
			case app.NodeKindConnection:
				projection, err := connectionProjection(ctx, transaction, child.id)
				if err != nil {
					return err
				}
				result.Connections = append(result.Connections, projection)
			default:
				return fmt.Errorf("invalid catalog child kind %q", child.kind)
			}
		}
		result.CatalogRevision, err = readCatalogRevision(ctx, transaction)
		return err
	})
	return result, err
}

func (repository *Repository) RenameFolder(ctx context.Context, request app.RenameFolderRequest) (result app.FolderResult, err error) {
	name, err := domain.NewName(request.Name)
	if err != nil {
		return result, err
	}
	err = repository.withImmediate(ctx, "rename folder", func(connection *sql.Conn) error {
		item, err := resolveSelector(ctx, connection, request.Folder)
		if err != nil {
			return err
		}
		if item.kind != app.NodeKindFolder {
			return ErrWrongKind
		}
		if err := rejectRoot(ctx, connection, item.id); err != nil {
			return err
		}
		current, err := folderProjection(ctx, connection, item.id)
		if err != nil {
			return err
		}
		if err := checkExpectedRevision(current.Revision, request.Expected); err != nil {
			return err
		}
		if name.String() != current.Name {
			if err := ensureNameAvailable(ctx, connection, string(current.ParentID), name.String(), item.id); err != nil {
				return err
			}
		}
		mutation, err := connection.ExecContext(ctx, `
			UPDATE nodes SET name = ?, revision = revision + 1, updated_at = ?
			WHERE id = ? AND revision = ?
		`, name.String(), formatCatalogTime(time.Now()), item.id, current.Revision)
		if err != nil {
			return fmt.Errorf("rename folder: %w", err)
		}
		if err := requireSingleMutation(mutation); err != nil {
			return err
		}
		result.CatalogRevision, err = incrementCatalogRevision(ctx, connection)
		if err != nil {
			return err
		}
		result.Folder, err = folderProjection(ctx, connection, item.id)
		return err
	})
	return result, err
}

func (repository *Repository) MoveFolder(ctx context.Context, request app.MoveFolderRequest) (result app.FolderResult, err error) {
	err = repository.withImmediate(ctx, "move folder", func(connection *sql.Conn) error {
		item, err := resolveSelector(ctx, connection, request.Folder)
		if err != nil {
			return err
		}
		if item.kind != app.NodeKindFolder {
			return ErrWrongKind
		}
		if err := rejectRoot(ctx, connection, item.id); err != nil {
			return err
		}
		destination, err := resolveSelector(ctx, connection, request.Destination)
		if err != nil {
			return err
		}
		if destination.kind != app.NodeKindFolder {
			return ErrWrongKind
		}
		current, err := folderProjection(ctx, connection, item.id)
		if err != nil {
			return err
		}
		destinationFolder, err := folderProjection(ctx, connection, destination.id)
		if err != nil {
			return err
		}
		if err := checkExpectedRevision(destinationFolder.Revision, request.ExpectedDestination); err != nil {
			return err
		}
		if err := checkExpectedPath(destinationFolder.Path, request.ExpectedDestinationPath); err != nil {
			return err
		}
		if err := checkExpectedRevision(current.Revision, request.Expected); err != nil {
			return err
		}
		if err := checkExpectedPath(current.Path, request.ExpectedSourcePath); err != nil {
			return err
		}
		subtree, err := readSubtree(ctx, connection, item.id)
		if err != nil {
			return err
		}
		for _, descendant := range subtree {
			if descendant.id == destination.id {
				return errors.Join(ErrConflict, ErrFolderCycle)
			}
		}
		if err := ensureNameAvailable(ctx, connection, destination.id, current.Name, item.id); err != nil {
			return err
		}
		mutation, err := connection.ExecContext(ctx, `
			UPDATE nodes SET parent_id = ?, revision = revision + 1, updated_at = ?
			WHERE id = ? AND revision = ?
		`, destination.id, formatCatalogTime(time.Now()), item.id, current.Revision)
		if err != nil {
			return fmt.Errorf("move folder: %w", err)
		}
		if err := requireSingleMutation(mutation); err != nil {
			return err
		}
		result.CatalogRevision, err = incrementCatalogRevision(ctx, connection)
		if err != nil {
			return err
		}
		result.Folder, err = folderProjection(ctx, connection, item.id)
		return err
	})
	return result, err
}

func (repository *Repository) DeleteFolder(ctx context.Context, request app.DeleteFolderRequest) (result app.DeleteFolderResult, err error) {
	err = repository.withImmediate(ctx, "delete folder", func(connection *sql.Conn) error {
		item, err := resolveSelector(ctx, connection, request.Folder)
		if err != nil {
			return err
		}
		if item.kind != app.NodeKindFolder {
			return ErrWrongKind
		}
		if err := rejectRoot(ctx, connection, item.id); err != nil {
			return err
		}
		current, err := folderProjection(ctx, connection, item.id)
		if err != nil {
			return err
		}
		if err := checkExpectedRevision(current.Revision, request.Expected); err != nil {
			return err
		}

		subtree, err := readSubtree(ctx, connection, item.id)
		if err != nil {
			return err
		}
		if !request.Recursive {
			if len(subtree) != 1 {
				return errors.Join(ErrConflict, ErrFolderNotEmpty)
			}
		} else if !matchesSnapshot(subtree, request.Snapshot) {
			return ErrConflict
		}

		sort.Slice(subtree, func(i, j int) bool {
			if subtree[i].depth != subtree[j].depth {
				return subtree[i].depth > subtree[j].depth
			}
			return subtree[i].id < subtree[j].id
		})
		for _, descendant := range subtree {
			mutation, err := connection.ExecContext(ctx, `DELETE FROM nodes WHERE id = ? AND revision = ?`, descendant.id, descendant.revision)
			if err != nil {
				return fmt.Errorf("delete folder subtree node: %w", err)
			}
			if err := requireSingleMutation(mutation); err != nil {
				return err
			}
			if descendant.kind == app.NodeKindFolder {
				result.FoldersDeleted++
			} else {
				result.ConnectionsDeleted++
			}
		}
		result.CatalogRevision, err = incrementCatalogRevision(ctx, connection)
		return err
	})
	return result, err
}

func folderProjection(ctx context.Context, query rowQuerier, id string) (app.Folder, error) {
	var projection app.Folder
	var parentID sql.NullString
	var kind, createdAt, updatedAt string
	if err := query.QueryRowContext(ctx, `
		SELECT id, parent_id, kind, name, revision, created_at, updated_at
		FROM nodes WHERE id = ? AND kind = 'folder'
	`, id).Scan(&projection.ID, &parentID, &kind, &projection.Name, &projection.Revision, &createdAt, &updatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return app.Folder{}, ErrNotFound
		}
		return app.Folder{}, fmt.Errorf("read folder: %w", err)
	}
	projection.ParentID = app.NodeID(parentID.String)
	projection.Kind = app.NodeKind(kind)
	var err error
	projection.CreatedAt, err = parseCatalogTime(createdAt)
	if err != nil {
		return app.Folder{}, err
	}
	projection.UpdatedAt, err = parseCatalogTime(updatedAt)
	if err != nil {
		return app.Folder{}, err
	}
	projection.Path, err = pathForNode(ctx, query, string(projection.ID))
	if err != nil {
		return app.Folder{}, err
	}
	return projection, nil
}

type subtreeNode struct {
	id       string
	kind     app.NodeKind
	revision app.Revision
	depth    int
}

type rowsQuerier interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func readSubtree(ctx context.Context, query rowsQuerier, root string) ([]subtreeNode, error) {
	rows, err := query.QueryContext(ctx, `
		WITH RECURSIVE subtree(id, kind, revision, depth, trail, cycle) AS (
			SELECT id, kind, revision, 0, '/' || id || '/', 0
			FROM nodes WHERE id = ?
			UNION ALL
			SELECT n.id, n.kind, n.revision, subtree.depth + 1,
			       subtree.trail || n.id || '/',
			       instr(subtree.trail, '/' || n.id || '/') > 0
			FROM nodes n JOIN subtree ON n.parent_id = subtree.id
			WHERE subtree.cycle = 0 AND subtree.depth < ?
		)
		SELECT id, kind, revision, depth, cycle,
		       EXISTS(SELECT 1 FROM nodes child WHERE child.parent_id = subtree.id)
		FROM subtree
	`, root, maxHierarchyDepth)
	if err != nil {
		return nil, fmt.Errorf("read folder subtree: %w", err)
	}
	defer rows.Close()

	result := make([]subtreeNode, 0)
	for rows.Next() {
		var node subtreeNode
		var cycle, hasChildren bool
		if err := rows.Scan(&node.id, &node.kind, &node.revision, &node.depth, &cycle, &hasChildren); err != nil {
			return nil, fmt.Errorf("scan folder subtree: %w", err)
		}
		if cycle {
			return nil, errors.Join(ErrConflict, ErrFolderCycle)
		}
		if node.depth == maxHierarchyDepth && hasChildren {
			return nil, errors.Join(ErrConflict, ErrHierarchyTooDeep)
		}
		result = append(result, node)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("read folder subtree: %w", err)
	}
	if len(result) == 0 {
		return nil, ErrNotFound
	}
	return result, nil
}

func matchesSnapshot(actual []subtreeNode, requested []app.NodeRevision) bool {
	if len(actual) != len(requested) {
		return false
	}
	expected := make(map[app.NodeID]app.Revision, len(requested))
	for _, item := range requested {
		if item.ID == "" || item.Revision == 0 {
			return false
		}
		if _, duplicate := expected[item.ID]; duplicate {
			return false
		}
		expected[item.ID] = item.Revision
	}
	for _, item := range actual {
		if expected[app.NodeID(item.id)] != item.revision {
			return false
		}
	}
	return true
}

func rejectRoot(ctx context.Context, query rowQuerier, id string) error {
	root, err := rootIDForQuery(ctx, query)
	if err != nil {
		return err
	}
	if id == root {
		return errors.Join(ErrConflict, ErrRootImmutable)
	}
	return nil
}
