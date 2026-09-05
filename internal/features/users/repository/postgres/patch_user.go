package users_postgres_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
	core_errors "github.com/1C-L0V3R/golang-td-app/internal/core/errors"
	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"
)

func (r *UsersRepository) PatchUser(
	ctx context.Context,
	id int,
	user domain.User,
) (domain.User, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout())
	defer cancel()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
	UPDATE todoapp.users
	SET
		full_name=$1,
		phone_number=$2,
		version=version+1
	WHERE id=$3 AND version=$4
	RETURNING
		id,
		version,
		full_name,
		phone_number;
	`

	row := tx.QueryRow(
		ctx,
		query,
		user.FullName,
		user.PhoneNumber,
		user.ID,
		user.Version,
	)

	var userModel UserModel
	err = row.Scan(
		&userModel.ID,
		&userModel.Version,
		&userModel.FullName,
		&userModel.PhoneNumber,
	)
	if err != nil {
		if errors.Is(err, core_postgres_pool.ErrNoRows) {
			return domain.User{}, fmt.Errorf(
				"user with id='%d' concurrently accessed: %w",
				id,
				core_errors.ErrConflict,
			)
		}

		return domain.User{}, fmt.Errorf("scan error: %w", err)
	}

	userDomain := domain.NewUser(
		userModel.ID,
		userModel.Version,
		userModel.FullName,
		userModel.PhoneNumber,
	)

	event, err := core_outbox.NewUserEvent(core_outbox.EventTypeUpdated, userDomain)
	if err != nil {
		return domain.User{}, fmt.Errorf("build outbox event: %w", err)
	}

	if err := r.outbox.InsertEvent(ctx, tx, event); err != nil {
		return domain.User{}, fmt.Errorf("insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit transaction: %w", err)
	}

	return userDomain, nil
}
