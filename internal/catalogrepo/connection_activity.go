package catalogrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/pluque01/orza/internal/app"
)

func (repository *Repository) ToggleConnectionFavourite(ctx context.Context, id app.NodeID) (bool, error) {
	var favourite bool
	err := repository.withImmediate(ctx, "toggle favourite", func(connection *sql.Conn) error {
		err := connection.QueryRowContext(ctx, `
			UPDATE connections SET favourite = 1 - favourite WHERE node_id = ?
			RETURNING favourite
		`, id).Scan(&favourite)
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return fmt.Errorf("update favourite: %w", err)
		}
		return nil
	})
	return favourite, err
}

func (repository *Repository) RecordConnectionOpened(ctx context.Context, id app.NodeID, openedAt time.Time) error {
	return repository.withImmediate(ctx, "record connection opening", func(connection *sql.Conn) error {
		// Keep the actual latest opening if concurrent sessions obtain the
		// write lock in a different order from their shell start times.
		mutation, err := connection.ExecContext(ctx, `
			UPDATE connections
			SET last_opened_at_ns = max(coalesce(last_opened_at_ns, ?), ?)
			WHERE node_id = ?
		`, openedAt.UnixNano(), openedAt.UnixNano(), id)
		if err != nil {
			return fmt.Errorf("record last opening: %w", err)
		}
		rows, err := mutation.RowsAffected()
		if err != nil {
			return fmt.Errorf("read last opening result: %w", err)
		}
		if rows == 0 {
			return ErrNotFound
		}
		return nil
	})
}
