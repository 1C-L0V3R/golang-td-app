package users_postgres_repository

import (
	"context"
	"errors"
	"fmt"

	core_errors "github.com/1C-L0V3R/golang-td-app/internal/core/errors"
	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"
)

func (r *UsersRepository) DeleteUser(
	ctx context.Context,
	id int,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout())
	defer cancel()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// RETURNING version: проектору нужна версия, под которой произошло
	// удаление, чтобы отбросить запоздавшие события того же агрегата.
	query := `
	DELETE FROM todoapp.users
	WHERE id=$1
	RETURNING id, version;
	`

	row := tx.QueryRow(ctx, query, id)

	var (
		deletedID      int
		deletedVersion int
	)

	if err := row.Scan(&deletedID, &deletedVersion); err != nil {
		if errors.Is(err, core_postgres_pool.ErrNoRows) {
			return fmt.Errorf(
				"user with id='%d': %w",
				id,
				core_errors.ErrNotFound,
			)
		}

		return fmt.Errorf("scan error: %w", err)
	}

	event, err := core_outbox.NewDeletedEvent(
		core_outbox.AggregateTypeUser,
		deletedID,
		deletedVersion,
	)
	if err != nil {
		return fmt.Errorf("build outbox event: %w", err)
	}

	if err := r.outbox.InsertEvent(ctx, tx, event); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}
