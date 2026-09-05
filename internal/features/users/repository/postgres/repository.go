package users_postgres_repository

import (
	"context"

	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"
)

type UsersRepository struct {
	pool   core_postgres_pool.Pool
	outbox OutboxRepository
}

type OutboxRepository interface {
	InsertEvent(
		ctx context.Context,
		q core_postgres_pool.Querier,
		event core_outbox.Event,
	) error
}

func NewUsersRepository(
	pool core_postgres_pool.Pool,
	outbox OutboxRepository,
) *UsersRepository {
	return &UsersRepository{
		pool:   pool,
		outbox: outbox,
	}
}
