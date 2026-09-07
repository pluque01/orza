package catalog

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/pluque01/orza/internal/domain"
	"golang.org/x/crypto/ssh"
)

var (
	ErrTrustedHostNotFound = errors.New("trusted host not found")
	ErrTrustedHostConflict = errors.New("trusted host revision conflict")
	ErrInvalidTrustedHost  = errors.New("invalid trusted host")
)

type TrustedHostRepository struct {
	store *Store
}

type HostEndpoint struct {
	CanonicalHost string
	Port          uint16
}

type PresentedHost struct {
	Endpoint          HostEndpoint
	KeyAlgorithm      string
	PublicKey         []byte
	FingerprintSHA256 string
}

type TrustedHost struct {
	ID string
	HostEndpoint
	KeyAlgorithm      string
	PublicKey         []byte
	FingerprintSHA256 string
	Revision          uint64
	AcceptedAt        time.Time
}

type TrustHostRequest struct {
	Host             PresentedHost
	ExpectedRevision *uint64
}

func NewTrustedHostRepository(store *Store) *TrustedHostRepository {
	return &TrustedHostRepository{store: store}
}

func (r *TrustedHostRepository) GetTrustedHost(ctx context.Context, endpoint HostEndpoint) (TrustedHost, error) {
	endpoint.CanonicalHost = canonicalHost(endpoint.CanonicalHost)
	if err := validateHostEndpoint(endpoint); err != nil {
		return TrustedHost{}, err
	}
	if r == nil || r.store == nil || r.store.db == nil {
		return TrustedHost{}, errors.New("catalog is not open")
	}

	host, err := scanTrustedHost(r.store.db.QueryRowContext(ctx, `
		SELECT id, canonical_host, port, key_algorithm, public_key,
		       fingerprint_sha256, revision, accepted_at
		FROM trusted_hosts
		WHERE canonical_host = ? AND port = ?
	`, endpoint.CanonicalHost, endpoint.Port))
	if errors.Is(err, sql.ErrNoRows) {
		return TrustedHost{}, fmt.Errorf("%w: %w", ErrTrustedHostNotFound, err)
	}
	if err != nil {
		return TrustedHost{}, fmt.Errorf("get trusted host: %w", err)
	}
	return host, nil
}

func (r *TrustedHostRepository) TrustHost(ctx context.Context, request TrustHostRequest) (trusted TrustedHost, err error) {
	presented, err := validatedPresentedHost(request.Host)
	if err != nil {
		return TrustedHost{}, err
	}
	if request.ExpectedRevision != nil && (*request.ExpectedRevision == 0 || uint64(*request.ExpectedRevision) > math.MaxInt64) {
		return TrustedHost{}, fmt.Errorf("%w: expected revision", ErrInvalidTrustedHost)
	}
	if r == nil || r.store == nil || r.store.db == nil {
		return TrustedHost{}, errors.New("catalog is not open")
	}

	conn, err := r.store.db.Conn(ctx)
	if err != nil {
		return TrustedHost{}, fmt.Errorf("acquire catalog connection: %w", err)
	}
	defer conn.Close()
	if _, err = conn.ExecContext(ctx, `BEGIN IMMEDIATE`); err != nil {
		return TrustedHost{}, fmt.Errorf("begin trusted host transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_, _ = conn.ExecContext(context.Background(), `ROLLBACK`)
		}
	}()

	if err = verifyTrustedHostWritableSchema(ctx, conn); err != nil {
		return TrustedHost{}, err
	}

	current, scanErr := scanTrustedHost(conn.QueryRowContext(ctx, `
		SELECT id, canonical_host, port, key_algorithm, public_key,
		       fingerprint_sha256, revision, accepted_at
		FROM trusted_hosts
		WHERE canonical_host = ? AND port = ?
	`, presented.Endpoint.CanonicalHost, presented.Endpoint.Port))
	switch {
	case errors.Is(scanErr, sql.ErrNoRows):
		if request.ExpectedRevision != nil {
			return TrustedHost{}, ErrTrustedHostConflict
		}
		trusted, err = insertTrustedHost(ctx, conn, presented)
	case scanErr != nil:
		return TrustedHost{}, fmt.Errorf("read trusted host for update: %w", scanErr)
	case sameHostKey(current, presented):
		trusted = current
	case request.ExpectedRevision == nil || current.Revision != *request.ExpectedRevision:
		return TrustedHost{}, ErrTrustedHostConflict
	case current.Revision >= uint64(math.MaxInt64):
		return TrustedHost{}, fmt.Errorf("%w: revision overflow", ErrInvalidTrustedHost)
	default:
		trusted, err = replaceTrustedHost(ctx, conn, current, presented)
	}
	if err != nil {
		return TrustedHost{}, err
	}

	if trusted.Revision != current.Revision || errors.Is(scanErr, sql.ErrNoRows) {
		result, updateErr := conn.ExecContext(ctx, `
			UPDATE catalog_meta
			SET catalog_revision = catalog_revision + 1
			WHERE singleton = 1 AND catalog_revision < ?
		`, int64(math.MaxInt64))
		if updateErr != nil {
			return TrustedHost{}, fmt.Errorf("increment catalog revision: %w", updateErr)
		}
		rows, rowsErr := result.RowsAffected()
		if rowsErr != nil {
			return TrustedHost{}, fmt.Errorf("inspect catalog revision update: %w", rowsErr)
		}
		if rows != 1 {
			return TrustedHost{}, fmt.Errorf("%w: catalog revision overflow", ErrInvalidTrustedHost)
		}
	}
	if _, err = conn.ExecContext(ctx, `COMMIT`); err != nil {
		return TrustedHost{}, fmt.Errorf("commit trusted host transaction: %w", err)
	}
	return trusted, nil
}

type rowScanner interface {
	Scan(...any) error
}

func scanTrustedHost(row rowScanner) (TrustedHost, error) {
	var (
		host       TrustedHost
		port       int64
		revision   int64
		acceptedAt string
	)
	if err := row.Scan(
		&host.ID,
		&host.CanonicalHost,
		&port,
		&host.KeyAlgorithm,
		&host.PublicKey,
		&host.FingerprintSHA256,
		&revision,
		&acceptedAt,
	); err != nil {
		return TrustedHost{}, err
	}
	if port < 1 || port > math.MaxUint16 || revision < 1 {
		return TrustedHost{}, fmt.Errorf("%w: corrupt persisted values", ErrInvalidTrustedHost)
	}
	parsedTime, err := time.Parse(time.RFC3339Nano, acceptedAt)
	if err != nil {
		return TrustedHost{}, fmt.Errorf("parse trusted host acceptance time: %w", err)
	}
	host.Port = uint16(port)
	host.Revision = uint64(revision)
	host.AcceptedAt = parsedTime
	host.PublicKey = bytes.Clone(host.PublicKey)
	return host, nil
}

func insertTrustedHost(ctx context.Context, conn *sql.Conn, presented PresentedHost) (TrustedHost, error) {
	id, err := domain.NewID()
	if err != nil {
		return TrustedHost{}, fmt.Errorf("create trusted host ID: %w", err)
	}
	acceptedAt := time.Now().UTC()
	trusted := TrustedHost{
		ID:                id.String(),
		HostEndpoint:      presented.Endpoint,
		KeyAlgorithm:      presented.KeyAlgorithm,
		PublicKey:         bytes.Clone(presented.PublicKey),
		FingerprintSHA256: presented.FingerprintSHA256,
		Revision:          1,
		AcceptedAt:        acceptedAt,
	}
	if _, err := conn.ExecContext(ctx, `
		INSERT INTO trusted_hosts(
			id, canonical_host, port, key_algorithm, public_key,
			fingerprint_sha256, revision, accepted_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, trusted.ID, trusted.CanonicalHost, trusted.Port, trusted.KeyAlgorithm,
		trusted.PublicKey, trusted.FingerprintSHA256, trusted.Revision,
		acceptedAt.Format(time.RFC3339Nano)); err != nil {
		return TrustedHost{}, fmt.Errorf("insert trusted host: %w", err)
	}
	return trusted, nil
}

func replaceTrustedHost(ctx context.Context, conn *sql.Conn, current TrustedHost, presented PresentedHost) (TrustedHost, error) {
	acceptedAt := time.Now().UTC()
	nextRevision := current.Revision + 1
	result, err := conn.ExecContext(ctx, `
		UPDATE trusted_hosts
		SET key_algorithm = ?, public_key = ?, fingerprint_sha256 = ?,
		    revision = ?, accepted_at = ?
		WHERE id = ? AND revision = ?
	`, presented.KeyAlgorithm, presented.PublicKey, presented.FingerprintSHA256,
		nextRevision, acceptedAt.Format(time.RFC3339Nano), current.ID, current.Revision)
	if err != nil {
		return TrustedHost{}, fmt.Errorf("replace trusted host: %w", err)
	}
	rows, err := result.RowsAffected()
	if err != nil {
		return TrustedHost{}, fmt.Errorf("inspect trusted host update: %w", err)
	}
	if rows != 1 {
		return TrustedHost{}, ErrTrustedHostConflict
	}
	current.KeyAlgorithm = presented.KeyAlgorithm
	current.PublicKey = bytes.Clone(presented.PublicKey)
	current.FingerprintSHA256 = presented.FingerprintSHA256
	current.Revision = nextRevision
	current.AcceptedAt = acceptedAt
	return current, nil
}

func verifyTrustedHostWritableSchema(ctx context.Context, conn *sql.Conn) error {
	var fileVersion, metadataVersion int
	if err := conn.QueryRowContext(ctx, `PRAGMA user_version`).Scan(&fileVersion); err != nil {
		return fmt.Errorf("read catalog schema version: %w", err)
	}
	if err := conn.QueryRowContext(ctx, `SELECT schema_version FROM catalog_meta WHERE singleton = 1`).Scan(&metadataVersion); err != nil {
		return fmt.Errorf("read catalog metadata schema version: %w", err)
	}
	if fileVersion > schemaVersion || metadataVersion > schemaVersion {
		return fmt.Errorf("%w: catalog schema version is %d/%d, supported version is %d", ErrNewerSchema, fileVersion, metadataVersion, schemaVersion)
	}
	if fileVersion != schemaVersion || metadataVersion != schemaVersion || fileVersion != metadataVersion {
		return fmt.Errorf("catalog schema version mismatch: file=%d metadata=%d supported=%d", fileVersion, metadataVersion, schemaVersion)
	}
	return nil
}

func validatedPresentedHost(host PresentedHost) (PresentedHost, error) {
	host.Endpoint.CanonicalHost = canonicalHost(host.Endpoint.CanonicalHost)
	if err := validateHostEndpoint(host.Endpoint); err != nil {
		return PresentedHost{}, err
	}
	key, err := ssh.ParsePublicKey(host.PublicKey)
	if err != nil {
		return PresentedHost{}, fmt.Errorf("%w: public key", ErrInvalidTrustedHost)
	}
	fingerprint := ssh.FingerprintSHA256(key)
	if host.KeyAlgorithm != key.Type() || host.FingerprintSHA256 != fingerprint {
		return PresentedHost{}, fmt.Errorf("%w: inconsistent public key metadata", ErrInvalidTrustedHost)
	}
	host.PublicKey = bytes.Clone(key.Marshal())
	return host, nil
}

func validateHostEndpoint(endpoint HostEndpoint) error {
	host := strings.TrimSpace(endpoint.CanonicalHost)
	if host == "" || strings.IndexByte(host, 0) >= 0 || endpoint.Port == 0 {
		return fmt.Errorf("%w: endpoint", ErrInvalidTrustedHost)
	}
	return nil
}

func canonicalHost(host string) string {
	return strings.TrimSuffix(strings.ToLower(strings.TrimSpace(host)), ".")
}

func sameHostKey(current TrustedHost, presented PresentedHost) bool {
	return current.KeyAlgorithm == presented.KeyAlgorithm &&
		current.FingerprintSHA256 == presented.FingerprintSHA256 &&
		bytes.Equal(current.PublicKey, presented.PublicKey)
}
