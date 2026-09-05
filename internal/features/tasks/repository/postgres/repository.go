package tasks_postgres_repository

import (
	"context"

	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"
)

type TasksRepository struct {
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

func NewTasksRepository(
	pool core_postgres_pool.Pool,
	outbox OutboxRepository,
) *TasksRepository {
	return &TasksRepository{
		pool:   pool,
		outbox: outbox,
	}
}
