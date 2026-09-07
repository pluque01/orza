CREATE TABLE catalog_meta (
    singleton INTEGER PRIMARY KEY CHECK (singleton = 1),
    catalog_id TEXT NOT NULL UNIQUE
        CHECK (length(catalog_id) = 32 AND catalog_id NOT GLOB '*[^0-9a-f]*'),
    root_id TEXT NOT NULL UNIQUE
        CHECK (length(root_id) = 32 AND root_id NOT GLOB '*[^0-9a-f]*'),
    catalog_revision INTEGER NOT NULL CHECK (catalog_revision >= 1),
    schema_version INTEGER NOT NULL CHECK (schema_version >= 1),
    created_at TEXT NOT NULL,
    FOREIGN KEY (root_id) REFERENCES nodes(id) ON DELETE RESTRICT
);

CREATE TABLE nodes (
    id TEXT PRIMARY KEY
        CHECK (length(id) = 32 AND id NOT GLOB '*[^0-9a-f]*'),
    parent_id TEXT REFERENCES nodes(id) ON DELETE RESTRICT,
    kind TEXT NOT NULL CHECK (kind IN ('folder', 'connection')),
    name TEXT NOT NULL
        CHECK (
            length(trim(name)) > 0
            AND instr(name, '/') = 0
            AND instr(name, char(0)) = 0
            AND name NOT GLOB ('*[' || char(1) || '-' || char(31) || char(127) || ']*')
        ),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,
    CHECK (parent_id IS NOT NULL OR kind = 'folder')
);

CREATE UNIQUE INDEX nodes_parent_name_uq ON nodes(parent_id, name);
CREATE UNIQUE INDEX nodes_single_root_uq ON nodes((1)) WHERE parent_id IS NULL;
CREATE INDEX nodes_parent_idx ON nodes(parent_id);
CREATE INDEX nodes_kind_idx ON nodes(kind);

CREATE TRIGGER nodes_parent_folder_insert
BEFORE INSERT ON nodes
WHEN NEW.parent_id IS NOT NULL
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM nodes WHERE id = NEW.parent_id AND kind = 'folder'
    ) THEN RAISE(ABORT, 'node parent must be a folder') END;
END;

CREATE TRIGGER nodes_parent_folder_update
BEFORE UPDATE OF parent_id ON nodes
WHEN NEW.parent_id IS NOT NULL
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM nodes WHERE id = NEW.parent_id AND kind = 'folder'
    ) THEN RAISE(ABORT, 'node parent must be a folder') END;
END;

CREATE TRIGGER nodes_folder_kind_immutable
BEFORE UPDATE OF kind ON nodes
WHEN NEW.kind <> OLD.kind
BEGIN
    SELECT RAISE(ABORT, 'node kind is immutable');
END;

CREATE TRIGGER nodes_folder_with_children
BEFORE UPDATE OF kind ON nodes
WHEN OLD.kind = 'folder' AND EXISTS (SELECT 1 FROM nodes WHERE parent_id = OLD.id)
BEGIN
    SELECT RAISE(ABORT, 'folder has children');
END;

CREATE TRIGGER nodes_root_immutable
BEFORE UPDATE OF parent_id, name, kind ON nodes
WHEN OLD.id = (SELECT root_id FROM catalog_meta WHERE singleton = 1)
BEGIN
    SELECT RAISE(ABORT, 'root node is immutable');
END;

CREATE TABLE connections (
    node_id TEXT PRIMARY KEY REFERENCES nodes(id) ON DELETE CASCADE,
    host TEXT NOT NULL CHECK (length(trim(host)) > 0 AND instr(host, char(0)) = 0),
    port INTEGER NOT NULL DEFAULT 22 CHECK (port BETWEEN 1 AND 65535),
    username TEXT,
    auth_method TEXT NOT NULL CHECK (auth_method IN ('agent', 'key', 'password')),
    identity_file TEXT,
    credential_ref TEXT
        CHECK (credential_ref IS NULL OR (length(credential_ref) = 32 AND credential_ref NOT GLOB '*[^0-9a-f]*')),
    CHECK (
        (auth_method = 'agent' AND identity_file IS NULL AND credential_ref IS NULL)
        OR (auth_method = 'key' AND length(trim(identity_file)) > 0 AND credential_ref IS NULL)
        OR (auth_method = 'password' AND identity_file IS NULL)
    )
);

CREATE TRIGGER connections_node_kind_insert
BEFORE INSERT ON connections
BEGIN
    SELECT CASE WHEN NOT EXISTS (
        SELECT 1 FROM nodes WHERE id = NEW.node_id AND kind = 'connection'
    ) THEN RAISE(ABORT, 'connection details require a connection node') END;
END;

CREATE TABLE trusted_hosts (
    id TEXT PRIMARY KEY
        CHECK (length(id) = 32 AND id NOT GLOB '*[^0-9a-f]*'),
    canonical_host TEXT NOT NULL CHECK (length(trim(canonical_host)) > 0 AND instr(canonical_host, char(0)) = 0),
    port INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    key_algorithm TEXT NOT NULL CHECK (length(trim(key_algorithm)) > 0),
    public_key BLOB NOT NULL CHECK (length(public_key) > 0),
    fingerprint_sha256 TEXT NOT NULL CHECK (length(trim(fingerprint_sha256)) > 0),
    revision INTEGER NOT NULL CHECK (revision >= 1),
    accepted_at TEXT NOT NULL
);

CREATE UNIQUE INDEX trusted_hosts_destination_uq ON trusted_hosts(canonical_host, port);

CREATE TABLE credential_operations (
    id TEXT PRIMARY KEY
        CHECK (length(id) = 32 AND id NOT GLOB '*[^0-9a-f]*'),
    connection_id TEXT NOT NULL REFERENCES connections(node_id) ON DELETE RESTRICT,
    expected_revision INTEGER NOT NULL CHECK (expected_revision >= 1),
    operation TEXT NOT NULL CHECK (operation IN ('save', 'replace', 'remove', 'delete_connection')),
    old_ref TEXT
        CHECK (old_ref IS NULL OR (length(old_ref) = 32 AND old_ref NOT GLOB '*[^0-9a-f]*')),
    new_ref TEXT
        CHECK (new_ref IS NULL OR (length(new_ref) = 32 AND new_ref NOT GLOB '*[^0-9a-f]*')),
    phase TEXT NOT NULL CHECK (phase IN ('prepared', 'secret_changed', 'catalog_changed', 'cleanup_pending')),
    target_auth_method TEXT CHECK (target_auth_method IS NULL OR target_auth_method IN ('agent', 'key', 'password')),
    target_identity_file TEXT,
    created_at TEXT NOT NULL,
    CHECK (
        target_auth_method IS NULL
        OR (target_auth_method = 'key' AND length(trim(target_identity_file)) > 0)
        OR (target_auth_method IN ('agent', 'password') AND target_identity_file IS NULL)
    )
);

CREATE INDEX credential_operations_connection_idx ON credential_operations(connection_id);
CREATE INDEX credential_operations_phase_idx ON credential_operations(phase);

INSERT INTO nodes(id, parent_id, kind, name, revision, created_at, updated_at)
VALUES (
    lower(hex(randomblob(16))),
    NULL,
    'folder',
    '.',
    1,
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now'),
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
);

INSERT INTO catalog_meta(singleton, catalog_id, root_id, catalog_revision, schema_version, created_at)
SELECT
    1,
    lower(hex(randomblob(16))),
    id,
    1,
    1,
    strftime('%Y-%m-%dT%H:%M:%fZ', 'now')
FROM nodes
WHERE parent_id IS NULL;
