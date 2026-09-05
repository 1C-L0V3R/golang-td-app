package users_postgres_repository

import (
	"context"
	"fmt"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
)

func (r *UsersRepository) CreateUser(
	ctx context.Context,
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
	INSERT INTO todoapp.users (full_name, phone_number)
	VALUES ($1, $2)
	RETURNING id, version, full_name, phone_number;
	`

	row := tx.QueryRow(ctx, query, user.FullName, user.PhoneNumber)

	var userModel UserModel
	err = row.Scan(
		&userModel.ID,
		&userModel.Version,
		&userModel.FullName,
		&userModel.PhoneNumber,
	)
	if err != nil {
		return domain.User{}, fmt.Errorf("scan error: %w", err)
	}

	userDomain := domain.NewUser(
		userModel.ID,
		userModel.Version,
		userModel.FullName,
		userModel.PhoneNumber,
	)

	event, err := core_outbox.NewUserEvent(core_outbox.EventTypeCreated, userDomain)
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
