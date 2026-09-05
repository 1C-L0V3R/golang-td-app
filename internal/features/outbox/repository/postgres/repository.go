package outbox_postgres_repository

import (
	"context"
	"fmt"
	"time"

	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"
)

type OutboxRepository struct {
	pool core_postgres_pool.Pool
}

func NewOutboxRepository(
	pool core_postgres_pool.Pool,
) *OutboxRepository {
	return &OutboxRepository{
		pool: pool,
	}
}

// FetchUnpublished забирает батч неопубликованных событий.
// FOR UPDATE SKIP LOCKED позволяет держать несколько реплик relay
// одновременно: каждая берёт свои строки, дублей нет.
func (r *OutboxRepository) FetchUnpublished(
	ctx context.Context,
	q core_postgres_pool.Querier,
	limit int,
) ([]core_outbox.Event, error) {
	query := `
	SELECT
		seq,
		id,
		aggregate_type,
		aggregate_id,
		event_type,
		version,
		payload,
		created_at
	FROM todoapp.outbox
	WHERE published_at IS NULL
	ORDER BY seq
	LIMIT $1
	FOR UPDATE SKIP LOCKED;
	`

	rows, err := q.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("query rows: %w", err)
	}
	defer rows.Close()

	var events []core_outbox.Event

	for rows.Next() {
		var (
			event         core_outbox.Event
			aggregateType string
			eventType     string
			payload       []byte
		)

		err := rows.Scan(
			&event.Seq,
			&event.ID,
			&aggregateType,
			&event.AggregateID,
			&eventType,
			&event.Version,
			&payload,
			&event.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("scan error: %w", err)
		}

		event.AggregateType = core_outbox.AggregateType(aggregateType)
		event.EventType = core_outbox.EventType(eventType)
		event.Payload = payload

		events = append(events, event)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return events, nil
}

func (r *OutboxRepository) MarkPublished(
	ctx context.Context,
	q core_postgres_pool.Querier,
	seqs []int64,
) error {
	if len(seqs) == 0 {
		return nil
	}

	query := `
	UPDATE todoapp.outbox
	SET published_at=now()
	WHERE seq = ANY($1);
	`

	if _, err := q.Exec(ctx, query, seqs); err != nil {
		return fmt.Errorf("exec query: %w", err)
	}

	return nil
}

// RecordFailure фиксирует неудачную попытку публикации. Вызывается вне
// транзакции чтения — та откатывается, чтобы снять блокировки строк.
func (r *OutboxRepository) RecordFailure(
	ctx context.Context,
	q core_postgres_pool.Querier,
	seqs []int64,
	reason string,
) error {
	if len(seqs) == 0 {
		return nil
	}

	query := `
	UPDATE todoapp.outbox
	SET
		attempts=attempts+1,
		last_error=$2
	WHERE seq = ANY($1);
	`

	if _, err := q.Exec(ctx, query, seqs, reason); err != nil {
		return fmt.Errorf("exec query: %w", err)
	}

	return nil
}

func (r *OutboxRepository) DeletePublishedBefore(
	ctx context.Context,
	q core_postgres_pool.Querier,
	before time.Time,
) (int64, error) {
	query := `
	DELETE FROM todoapp.outbox
	WHERE published_at IS NOT NULL AND published_at < $1;
	`

	cmdTag, err := q.Exec(ctx, query, before)
	if err != nil {
		return 0, fmt.Errorf("exec query: %w", err)
	}

	return cmdTag.RowsAffected(), nil
}
