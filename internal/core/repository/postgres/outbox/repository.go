package core_outbox_repository

import (
	"context"
	"fmt"

	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"
)

type OutboxRepository struct{}

func NewOutboxRepository() *OutboxRepository {
	return &OutboxRepository{}
}

// InsertEvent принимает Querier, а не пул: вызывается с транзакцией того
// репозитория, который меняет агрегат, — в этом и состоит суть паттерна.
func (r *OutboxRepository) InsertEvent(
	ctx context.Context,
	q core_postgres_pool.Querier,
	event core_outbox.Event,
) error {
	query := `
	INSERT INTO todoapp.outbox (
		id,
		aggregate_type,
		aggregate_id,
		event_type,
		version,
		payload,
		created_at
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7);
	`

	_, err := q.Exec(
		ctx,
		query,
		event.ID,
		string(event.AggregateType),
		event.AggregateID,
		string(event.EventType),
		event.Version,
		[]byte(event.Payload),
		event.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("exec query: %w", err)
	}

	return nil
}
