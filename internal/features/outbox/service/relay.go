package outbox_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	core_kafka "github.com/1C-L0V3R/golang-td-app/internal/core/broker/kafka"
	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"
	"go.uber.org/zap"
)

// RelayService вычитывает outbox-таблицу и публикует события в Kafka.
// Гарантия доставки — at-least-once: если публикация прошла, а COMMIT
// не успел, событие уйдёт повторно. Идемпотентность на стороне проекции.
type RelayService struct {
	config     Config
	pool       core_postgres_pool.Pool
	repository OutboxRepository
	producer   Producer
	topics     Topics
	log        *core_logger.Logger
}

type OutboxRepository interface {
	FetchUnpublished(
		ctx context.Context,
		q core_postgres_pool.Querier,
		limit int,
	) ([]core_outbox.Event, error)

	MarkPublished(
		ctx context.Context,
		q core_postgres_pool.Querier,
		seqs []int64,
	) error

	RecordFailure(
		ctx context.Context,
		q core_postgres_pool.Querier,
		seqs []int64,
		reason string,
	) error

	DeletePublishedBefore(
		ctx context.Context,
		q core_postgres_pool.Querier,
		before time.Time,
	) (int64, error)
}

type Producer interface {
	Publish(ctx context.Context, messages ...core_kafka.Message) error
}

type Topics struct {
	Users string
	Tasks string
}

func NewRelayService(
	config Config,
	pool core_postgres_pool.Pool,
	repository OutboxRepository,
	producer Producer,
	topics Topics,
	log *core_logger.Logger,
) *RelayService {
	return &RelayService{
		config:     config,
		pool:       pool,
		repository: repository,
		producer:   producer,
		topics:     topics,
		log:        log,
	}
}

const (
	maxBackoff      = 30 * time.Second
	cleanupInterval = time.Hour
)

func (s *RelayService) Run(ctx context.Context) error {
	s.log.Warn(
		"start outbox relay",
		zap.Duration("poll_interval", s.config.PollInterval),
		zap.Int("batch_size", s.config.BatchSize),
	)

	cleanupTicker := time.NewTicker(cleanupInterval)
	defer cleanupTicker.Stop()

	backoff := time.Duration(0)

	for {
		select {
		case <-ctx.Done():
			s.log.Warn("outbox relay stopped")

			return nil
		case <-cleanupTicker.C:
			s.cleanup(ctx)
		case <-time.After(s.delay(backoff)):
			published, err := s.publishBatch(ctx)
			if err != nil {
				if ctx.Err() != nil {
					s.log.Warn("outbox relay stopped")

					return nil
				}

				backoff = nextBackoff(backoff)

				s.log.Error(
					"outbox relay batch failed",
					zap.Error(err),
					zap.Duration("retry_in", backoff),
				)

				continue
			}

			backoff = 0

			// Батч заполнен целиком — вероятно, есть ещё. Не ждём тикер.
			if published == s.config.BatchSize {
				continue
			}
		}
	}
}

// delay возвращает паузу перед следующей итерацией: обычный интервал
// опроса либо backoff после ошибки.
func (s *RelayService) delay(backoff time.Duration) time.Duration {
	if backoff > 0 {
		return backoff
	}

	return s.config.PollInterval
}

func nextBackoff(current time.Duration) time.Duration {
	if current == 0 {
		return time.Second
	}

	next := current * 2
	if next > maxBackoff {
		return maxBackoff
	}

	return next
}

func (s *RelayService) publishBatch(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, s.pool.OpTimeout())
	defer cancel()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	events, err := s.repository.FetchUnpublished(ctx, tx, s.config.BatchSize)
	if err != nil {
		return 0, fmt.Errorf("fetch unpublished events: %w", err)
	}

	if len(events) == 0 {
		return 0, nil
	}

	messages := make([]core_kafka.Message, 0, len(events))
	seqs := make([]int64, 0, len(events))

	for _, event := range events {
		message, err := s.message(event)
		if err != nil {
			// Событие невозможно опубликовать в принципе: помечаем как
			// отправленное, иначе оно навсегда заблокирует очередь.
			s.log.Error(
				"skip malformed outbox event",
				zap.Int64("seq", event.Seq),
				zap.String("id", event.ID.String()),
				zap.Error(err),
			)

			seqs = append(seqs, event.Seq)

			continue
		}

		messages = append(messages, message)
		seqs = append(seqs, event.Seq)
	}

	if err := s.producer.Publish(ctx, messages...); err != nil {
		s.recordFailure(ctx, tx, seqs, err)

		return 0, fmt.Errorf("publish events: %w", err)
	}

	if err := s.repository.MarkPublished(ctx, tx, seqs); err != nil {
		return 0, fmt.Errorf("mark events published: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit transaction: %w", err)
	}

	s.log.Debug(
		"outbox events published",
		zap.Int("count", len(messages)),
	)

	return len(events), nil
}

func (s *RelayService) message(event core_outbox.Event) (core_kafka.Message, error) {
	var topic string

	switch event.AggregateType {
	case core_outbox.AggregateTypeUser:
		topic = s.topics.Users
	case core_outbox.AggregateTypeTask:
		topic = s.topics.Tasks
	default:
		return core_kafka.Message{}, fmt.Errorf(
			"unknown aggregate type: %q",
			event.AggregateType,
		)
	}

	value, err := json.Marshal(event.Envelope())
	if err != nil {
		return core_kafka.Message{}, fmt.Errorf("marshal envelope: %w", err)
	}

	return core_kafka.Message{
		Topic: topic,
		Key:   []byte(event.AggregateID),
		Value: value,
	}, nil
}

// recordFailure пишет причину сбоя отдельной транзакцией: транзакция
// чтения откатывается, чтобы освободить блокировки строк.
func (s *RelayService) recordFailure(
	ctx context.Context,
	tx core_postgres_pool.Tx,
	seqs []int64,
	reason error,
) {
	_ = tx.Rollback(ctx)

	if errors.Is(ctx.Err(), context.Canceled) {
		return
	}

	failureCtx, cancel := context.WithTimeout(
		context.WithoutCancel(ctx),
		s.pool.OpTimeout(),
	)
	defer cancel()

	err := s.repository.RecordFailure(failureCtx, s.pool, seqs, reason.Error())
	if err != nil {
		s.log.Error("failed to record outbox failure", zap.Error(err))
	}
}

func (s *RelayService) cleanup(ctx context.Context) {
	ctx, cancel := context.WithTimeout(ctx, s.pool.OpTimeout())
	defer cancel()

	before := time.Now().Add(-s.config.Retention)

	deleted, err := s.repository.DeletePublishedBefore(ctx, s.pool, before)
	if err != nil {
		s.log.Error("failed to cleanup published outbox events", zap.Error(err))

		return
	}

	if deleted > 0 {
		s.log.Debug(
			"published outbox events cleaned up",
			zap.Int64("count", deleted),
		)
	}
}
