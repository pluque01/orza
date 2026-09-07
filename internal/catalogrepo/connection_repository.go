package catalogrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/pluque01/orza/internal/app"
	"github.com/pluque01/orza/internal/catalog"
	"github.com/pluque01/orza/internal/domain"
)

var (
	ErrNotFound        = app.ErrNotFound
	ErrConflict        = app.ErrConflict
	ErrWrongKind       = errors.Join(app.ErrInvalidRequest, errors.New("catalog item has the wrong kind"))
	ErrInvalidSelector = errors.Join(app.ErrInvalidRequest, errors.New("invalid item selector"))
)

// Repository persists catalog aggregates using a Store's configured SQLite connection.
type Repository struct {
	store *catalog.Store
}

var _ app.ConnectionRepository = (*Repository)(nil)

func NewRepository(store *catalog.Store) *Repository {
	return &Repository{store: store}
}

func (repository *Repository) CreateConnection(ctx context.Context, request app.CreateConnectionRequest) (result app.ConnectionResult, err error) {
	name, err := domain.NewName(request.Name)
	if err != nil {
		return result, err
	}
	details, err := newConnectionDetails(request.Host, request.Port, request.Username, request.AuthMethod, request.IdentityFile, "", false)
	if err != nil {
		return result, err
	}
	id, err := domain.NewID()
	if err != nil {
		return result, fmt.Errorf("generate connection ID: %w", err)
	}

	err = repository.withImmediate(ctx, "create connection", func(connection *sql.Conn) error {
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
			VALUES(?, ?, 'connection', ?, 1, ?, ?)
		`, id.String(), parent.id, name.String(), now, now); err != nil {
			return fmt.Errorf("insert connection node: %w", err)
		}
		if err := insertConnectionDetails(ctx, connection, id.String(), details); err != nil {
			return err
		}
		result.CatalogRevision, err = incrementCatalogRevision(ctx, connection)
		if err != nil {
			return err
		}
		result.Connection, err = connectionProjection(ctx, connection, id.String())
		return err
	})
	return result, err
}

func (repository *Repository) GetConnection(ctx context.Context, selector app.ItemSelector) (result app.ConnectionResult, err error) {
	err = repository.withRead(ctx, func(transaction *sql.Tx) error {
		item, err := resolveSelector(ctx, transaction, selector)
		if err != nil {
			return err
		}
		if item.kind != app.NodeKindConnection {
			return ErrWrongKind
		}
		result.Connection, err = connectionProjection(ctx, transaction, item.id)
		if err != nil {
			return err
		}
		result.CatalogRevision, err = readCatalogRevision(ctx, transaction)
		return err
	})
	return result, err
}

func (repository *Repository) ListConnections(ctx context.Context, request app.ListConnectionsRequest) (result app.ListConnectionsResult, err error) {
	err = repository.withRead(ctx, func(transaction *sql.Tx) error {
		folder, err := resolveSelector(ctx, transaction, request.Folder)
		if err != nil {
			return err
		}
		if folder.kind != app.NodeKindFolder {
			return ErrWrongKind
		}

		rows, err := transaction.QueryContext(ctx, `
			SELECT id FROM nodes
			WHERE parent_id = ? AND kind = 'connection'
			ORDER BY name COLLATE BINARY, id
		`, folder.id)
		if err != nil {
			return fmt.Errorf("list connections: %w", err)
		}
		var ids []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				_ = rows.Close()
				return fmt.Errorf("scan connection ID: %w", err)
			}
			ids = append(ids, id)
		}
		if err := rows.Close(); err != nil {
			return fmt.Errorf("close connection list: %w", err)
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("list connections: %w", err)
		}

		result.Connections = make([]app.Connection, 0, len(ids))
		for _, id := range ids {
			projection, err := connectionProjection(ctx, transaction, id)
			if err != nil {
				return err
			}
			result.Connections = append(result.Connections, projection)
		}
		result.CatalogRevision, err = readCatalogRevision(ctx, transaction)
		return err
	})
	return result, err
}

func (repository *Repository) UpdateConnection(ctx context.Context, request app.UpdateConnectionRequest) (result app.ConnectionResult, err error) {
	if request.Port != nil && *request.Port == 0 {
		return result, &domain.ValidationError{Field: "port", Err: domain.ErrInvalidPort}
	}
	if request.Name != nil {
		if _, err := domain.NewName(*request.Name); err != nil {
			return result, err
		}
	}

	err = repository.withImmediate(ctx, "update connection", func(connection *sql.Conn) error {
		item, err := resolveSelector(ctx, connection, request.Connection)
		if err != nil {
			return err
		}
		if item.kind != app.NodeKindConnection {
			return ErrWrongKind
		}
		current, err := connectionProjection(ctx, connection, item.id)
		if err != nil {
			return err
		}
		if err := checkExpectedRevision(current.Revision, request.Expected); err != nil {
			return err
		}

		name := current.Name
		if request.Name != nil {
			name = *request.Name
			if name != current.Name {
				if err := ensureNameAvailable(ctx, connection, string(current.ParentID), name, item.id); err != nil {
					return err
				}
			}
		}
		host := current.Host
		if request.Host != nil {
			host = *request.Host
		}
		port := current.Port
		if request.Port != nil {
			port = *request.Port
		}
		username := current.Username
		if request.Username != nil {
			username = *request.Username
		}
		authMethod := current.AuthMethod
		if request.AuthMethod != nil {
			authMethod = *request.AuthMethod
		}
		identityFile := current.IdentityFile
		if request.IdentityFile != nil {
			identityFile = *request.IdentityFile
		}
		details, err := newConnectionDetails(host, port, username, authMethod, identityFile, current.CredentialRef, true)
		if err != nil {
			return err
		}

		now := formatCatalogTime(time.Now())
		mutation, err := connection.ExecContext(ctx, `
			UPDATE nodes
			SET name = ?, revision = revision + 1, updated_at = ?
			WHERE id = ? AND revision = ?
		`, name, now, item.id, current.Revision)
		if err != nil {
			return fmt.Errorf("update connection node: %w", err)
		}
		if err := requireSingleMutation(mutation); err != nil {
			return err
		}
		if _, err := connection.ExecContext(ctx, `
			UPDATE connections
			SET host = ?, port = ?, username = ?, auth_method = ?, identity_file = ?, credential_ref = ?
			WHERE node_id = ?
		`, details.Host(), details.Port(), nullable(details.Username()), details.AuthMethod().String(), nullable(details.IdentityFile()), nullable(details.CredentialRef()), item.id); err != nil {
			return fmt.Errorf("update connection details: %w", err)
		}
		result.CatalogRevision, err = incrementCatalogRevision(ctx, connection)
		if err != nil {
			return err
		}
		result.Connection, err = connectionProjection(ctx, connection, item.id)
		return err
	})
	return result, err
}

func (repository *Repository) MoveConnection(ctx context.Context, request app.MoveConnectionRequest) (result app.ConnectionResult, err error) {
	err = repository.withImmediate(ctx, "move connection", func(connection *sql.Conn) error {
		item, err := resolveSelector(ctx, connection, request.Connection)
		if err != nil {
			return err
		}
		if item.kind != app.NodeKindConnection {
			return ErrWrongKind
		}
		destination, err := resolveSelector(ctx, connection, request.Destination)
		if err != nil {
			return err
		}
		if destination.kind != app.NodeKindFolder {
			return ErrWrongKind
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
		current, err := connectionProjection(ctx, connection, item.id)
		if err != nil {
			return err
		}
		if err := checkExpectedRevision(current.Revision, request.Expected); err != nil {
			return err
		}
		if err := checkExpectedPath(current.Path, request.ExpectedSourcePath); err != nil {
			return err
		}
		if err := ensureNameAvailable(ctx, connection, destination.id, current.Name, item.id); err != nil {
			return err
		}

		mutation, err := connection.ExecContext(ctx, `
			UPDATE nodes
			SET parent_id = ?, revision = revision + 1, updated_at = ?
			WHERE id = ? AND revision = ?
		`, destination.id, formatCatalogTime(time.Now()), item.id, current.Revision)
		if err != nil {
			return fmt.Errorf("move connection: %w", err)
		}
		if err := requireSingleMutation(mutation); err != nil {
			return err
		}
		result.CatalogRevision, err = incrementCatalogRevision(ctx, connection)
		if err != nil {
			return err
		}
		result.Connection, err = connectionProjection(ctx, connection, item.id)
		return err
	})
	return result, err
}

func (repository *Repository) DeleteConnection(ctx context.Context, request app.DeleteConnectionRequest) (result app.DeleteConnectionResult, err error) {
	err = repository.withImmediate(ctx, "delete connection", func(connection *sql.Conn) error {
		item, err := resolveSelector(ctx, connection, request.Connection)
		if err != nil {
			return err
		}
		if item.kind != app.NodeKindConnection {
			return ErrWrongKind
		}
		result.Deleted, err = connectionProjection(ctx, connection, item.id)
		if err != nil {
			return err
		}
		if err := checkExpectedRevision(result.Deleted.Revision, request.Expected); err != nil {
			return err
		}
		mutation, err := connection.ExecContext(ctx, `DELETE FROM nodes WHERE id = ? AND revision = ?`, item.id, result.Deleted.Revision)
		if err != nil {
			return fmt.Errorf("delete connection: %w", err)
		}
		if err := requireSingleMutation(mutation); err != nil {
			return err
		}
		result.CatalogRevision, err = incrementCatalogRevision(ctx, connection)
		return err
	})
	return result, err
}

type rowQuerier interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

type catalogItem struct {
	id   string
	kind app.NodeKind
}

func (repository *Repository) withRead(ctx context.Context, operation func(*sql.Tx) error) (err error) {
	if repository == nil || repository.store == nil || repository.store.DB() == nil {
		return errors.New("catalog is not open")
	}
	transaction, err := repository.store.DB().BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return fmt.Errorf("begin catalog read: %w", err)
	}
	defer func() {
		if err != nil {
			_ = transaction.Rollback()
		}
	}()
	if err = operation(transaction); err != nil {
		return err
	}
	if err = transaction.Commit(); err != nil {
		return fmt.Errorf("commit catalog read: %w", err)
	}
	return nil
}

func (repository *Repository) withImmediate(ctx context.Context, operation string, mutate func(*sql.Conn) error) (err error) {
	if repository == nil || repository.store == nil || repository.store.DB() == nil {
		return errors.New("catalog is not open")
	}
	connection, err := repository.store.DB().Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire catalog connection: %w", err)
	}
	defer connection.Close()
	if _, err = connection.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return fmt.Errorf("begin %s: %w", operation, err)
	}
	defer func() {
		if err != nil {
			_, _ = connection.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()
	if err = repository.store.CheckWritableSchema(ctx, connection); err != nil {
		return err
	}
	if err = mutate(connection); err != nil {
		return err
	}
	if _, err = connection.ExecContext(ctx, `COMMIT`); err != nil {
		return fmt.Errorf("commit %s: %w", operation, err)
	}
	return nil
}

func resolveSelector(ctx context.Context, query rowQuerier, selector app.ItemSelector) (catalogItem, error) {
	if (selector.ID == "") == (selector.Path == "") {
		return catalogItem{}, ErrInvalidSelector
	}
	if selector.ID != "" {
		id, err := domain.ParseID(string(selector.ID))
		if err != nil {
			return catalogItem{}, err
		}
		return itemByID(ctx, query, id.String())
	}

	path, err := domain.ParseLogicalPath(selector.Path)
	if err != nil {
		return catalogItem{}, err
	}
	var current catalogItem
	if err := query.QueryRowContext(ctx, `
		SELECT nodes.id, nodes.kind
		FROM catalog_meta JOIN nodes ON nodes.id = catalog_meta.root_id
		WHERE catalog_meta.singleton = 1
	`).Scan(&current.id, &current.kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return catalogItem{}, ErrNotFound
		}
		return catalogItem{}, fmt.Errorf("resolve root: %w", err)
	}
	for _, segment := range path.Segments() {
		if current.kind != app.NodeKindFolder {
			return catalogItem{}, ErrNotFound
		}
		if err := query.QueryRowContext(ctx, `
			SELECT id, kind FROM nodes WHERE parent_id = ? AND name = ? COLLATE BINARY
		`, current.id, segment.String()).Scan(&current.id, &current.kind); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return catalogItem{}, ErrNotFound
			}
			return catalogItem{}, fmt.Errorf("resolve catalog path: %w", err)
		}
	}
	return current, nil
}

func itemByID(ctx context.Context, query rowQuerier, id string) (catalogItem, error) {
	var item catalogItem
	if err := query.QueryRowContext(ctx, `SELECT id, kind FROM nodes WHERE id = ?`, id).Scan(&item.id, &item.kind); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return catalogItem{}, ErrNotFound
		}
		return catalogItem{}, fmt.Errorf("resolve catalog ID: %w", err)
	}
	return item, nil
}

func connectionProjection(ctx context.Context, query rowQuerier, id string) (app.Connection, error) {
	var projection app.Connection
	var parentID, username, identityFile, credentialRef sql.NullString
	var kind, authMethod string
	var createdAt, updatedAt string
	if err := query.QueryRowContext(ctx, `
		SELECT n.id, n.parent_id, n.kind, n.name, n.revision, n.created_at, n.updated_at,
		       c.host, c.port, c.username, c.auth_method, c.identity_file, c.credential_ref
		FROM nodes n JOIN connections c ON c.node_id = n.id
		WHERE n.id = ?
	`, id).Scan(
		&projection.ID, &parentID, &kind, &projection.Name, &projection.Revision, &createdAt, &updatedAt,
		&projection.Host, &projection.Port, &username, &authMethod, &identityFile, &credentialRef,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return app.Connection{}, ErrNotFound
		}
		return app.Connection{}, fmt.Errorf("read connection: %w", err)
	}
	projection.ParentID = app.NodeID(parentID.String)
	projection.Kind = app.NodeKind(kind)
	projection.Username = username.String
	projection.AuthMethod = app.AuthMethod(authMethod)
	projection.IdentityFile = identityFile.String
	projection.CredentialRef = credentialRef.String

	var err error
	projection.CreatedAt, err = parseCatalogTime(createdAt)
	if err != nil {
		return app.Connection{}, err
	}
	projection.UpdatedAt, err = parseCatalogTime(updatedAt)
	if err != nil {
		return app.Connection{}, err
	}
	projection.Path, err = pathForNode(ctx, query, string(projection.ID))
	if err != nil {
		return app.Connection{}, err
	}
	if _, err := newConnectionDetails(projection.Host, projection.Port, projection.Username, projection.AuthMethod, projection.IdentityFile, projection.CredentialRef, true); err != nil {
		return app.Connection{}, fmt.Errorf("invalid persisted connection: %w", err)
	}
	return projection, nil
}

func pathForNode(ctx context.Context, query rowQuerier, id string) (string, error) {
	root, err := rootIDForQuery(ctx, query)
	if err != nil {
		return "", err
	}
	var reversed []string
	visited := make(map[string]struct{})
	current := id
	for current != root {
		if _, exists := visited[current]; exists || len(visited) >= 100 {
			return "", errors.New("catalog hierarchy is cyclic or too deep")
		}
		visited[current] = struct{}{}
		var parent sql.NullString
		var name string
		if err := query.QueryRowContext(ctx, `SELECT parent_id, name FROM nodes WHERE id = ?`, current).Scan(&parent, &name); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return "", ErrNotFound
			}
			return "", fmt.Errorf("build catalog path: %w", err)
		}
		if !parent.Valid {
			return "", errors.New("catalog node is not descended from root")
		}
		reversed = append(reversed, name)
		current = parent.String
	}
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	return "/" + strings.Join(reversed, "/"), nil
}

func rootIDForQuery(ctx context.Context, query rowQuerier) (string, error) {
	var root string
	if err := query.QueryRowContext(ctx, `SELECT root_id FROM catalog_meta WHERE singleton = 1`).Scan(&root); err != nil {
		return "", fmt.Errorf("read catalog root: %w", err)
	}
	return root, nil
}

func newConnectionDetails(host string, port uint16, username string, authMethod app.AuthMethod, identityFile, credentialRef string, explicitPort bool) (domain.ConnectionDetails, error) {
	if explicitPort && port == 0 {
		return domain.ConnectionDetails{}, &domain.ValidationError{Field: "port", Err: domain.ErrInvalidPort}
	}
	method, err := domain.ParseAuthMethod(string(authMethod))
	if err != nil {
		return domain.ConnectionDetails{}, err
	}
	return domain.NewConnectionDetails(host, uint64(port), username, method, identityFile, credentialRef)
}

func insertConnectionDetails(ctx context.Context, connection *sql.Conn, id string, details domain.ConnectionDetails) error {
	if _, err := connection.ExecContext(ctx, `
		INSERT INTO connections(node_id, host, port, username, auth_method, identity_file, credential_ref)
		VALUES(?, ?, ?, ?, ?, ?, ?)
	`, id, details.Host(), details.Port(), nullable(details.Username()), details.AuthMethod().String(), nullable(details.IdentityFile()), nullable(details.CredentialRef())); err != nil {
		return fmt.Errorf("insert connection details: %w", err)
	}
	return nil
}

func ensureNameAvailable(ctx context.Context, query rowQuerier, parentID, name, exceptID string) error {
	var existing string
	err := query.QueryRowContext(ctx, `
		SELECT id FROM nodes
		WHERE parent_id = ? AND name = ? COLLATE BINARY AND id <> ?
	`, parentID, name, exceptID).Scan(&existing)
	if err == nil {
		return ErrConflict
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return fmt.Errorf("check catalog name: %w", err)
}

func checkExpectedRevision(actual app.Revision, expected *app.Revision) error {
	if expected != nil && *expected != actual {
		return ErrConflict
	}
	return nil
}

func checkExpectedPath(actual, expected string) error {
	if expected != "" && expected != actual {
		return ErrConflict
	}
	return nil
}

func requireSingleMutation(result sql.Result) error {
	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read mutation result: %w", err)
	}
	if rows != 1 {
		return ErrConflict
	}
	return nil
}

func incrementCatalogRevision(ctx context.Context, connection *sql.Conn) (app.CatalogRevision, error) {
	if _, err := connection.ExecContext(ctx, `
		UPDATE catalog_meta SET catalog_revision = catalog_revision + 1 WHERE singleton = 1
	`); err != nil {
		return 0, fmt.Errorf("increment catalog revision: %w", err)
	}
	return readCatalogRevision(ctx, connection)
}

func readCatalogRevision(ctx context.Context, query rowQuerier) (app.CatalogRevision, error) {
	var revision app.CatalogRevision
	if err := query.QueryRowContext(ctx, `SELECT catalog_revision FROM catalog_meta WHERE singleton = 1`).Scan(&revision); err != nil {
		return 0, fmt.Errorf("read catalog revision: %w", err)
	}
	return revision, nil
}

func nullable(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func formatCatalogTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseCatalogTime(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse catalog timestamp: %w", err)
	}
	return parsed, nil
}
