package projection_service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	core_kafka "github.com/1C-L0V3R/golang-td-app/internal/core/broker/kafka"
	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	projection_mongo_repository "github.com/1C-L0V3R/golang-td-app/internal/features/projection/repository/mongo"
	"github.com/segmentio/kafka-go"
	"go.mongodb.org/mongo-driver/bson"
	"go.uber.org/zap"
)

// ProjectionService читает события из Kafka и поддерживает в MongoDB
// актуальную копию users/tasks. Оффсет коммитится только после успешной
// записи в Mongo: at-least-once доставка плюс идемпотентный upsert дают
// корректную проекцию даже при повторах.
type ProjectionService struct {
	reader     *kafka.Reader
	repository ProjectionRepository
	log        *core_logger.Logger
}

type ProjectionRepository interface {
	Upsert(
		ctx context.Context,
		collection string,
		id int,
		version int,
		document bson.M,
	) error

	Delete(
		ctx context.Context,
		collection string,
		id int,
		version int,
	) error
}

func NewProjectionService(
	config Config,
	kafkaConfig core_kafka.Config,
	repository ProjectionRepository,
	log *core_logger.Logger,
) *ProjectionService {
	reader := kafka.NewReader(kafka.ReaderConfig{
		Brokers: kafkaConfig.Brokers,
		GroupID: config.ConsumerGroup,
		GroupTopics: []string{
			kafkaConfig.UsersTopic(),
			kafkaConfig.TasksTopic(),
		},
	})

	return &ProjectionService{
		reader:     reader,
		repository: repository,
		log:        log,
	}
}

const retryDelay = time.Second

func (s *ProjectionService) Run(ctx context.Context) error {
	s.log.Warn("start projection consumer")

	for {
		message, err := s.reader.FetchMessage(ctx)
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, io.EOF) {
				s.log.Warn("projection consumer stopped")

				return nil
			}

			// Брокер может быть недоступен временно (рестарт, ребаланс) —
			// консьюмер обязан это переждать, а не завершаться.
			s.log.Error("failed to fetch kafka message", zap.Error(err))

			select {
			case <-ctx.Done():
				s.log.Warn("projection consumer stopped")

				return nil
			case <-time.After(retryDelay):
			}

			continue
		}

		if err := s.handle(ctx, message); err != nil {
			if ctx.Err() != nil {
				s.log.Warn("projection consumer stopped")

				return nil
			}

			// Оффсет не коммитим: сообщение будет прочитано cнова.
			s.log.Error(
				"failed to project message",
				zap.String("topic", message.Topic),
				zap.Int("partition", message.Partition),
				zap.Int64("offset", message.Offset),
				zap.Error(err),
			)

			select {
			case <-ctx.Done():
				return nil
			case <-time.After(retryDelay):
			}

			continue
		}

		if err := s.reader.CommitMessages(ctx, message); err != nil {
			if ctx.Err() != nil {
				s.log.Warn("projection consumer stopped")

				return nil
			}

			// Оффсет не закоммичен: сообщение придёт снова, а повторная
			// проекция идемпотентна — расхождения не будет.
			s.log.Error(
				"failed to commit kafka message",
				zap.String("topic", message.Topic),
				zap.Int64("offset", message.Offset),
				zap.Error(err),
			)

			select {
			case <-ctx.Done():
				s.log.Warn("projection consumer stopped")

				return nil
			case <-time.After(retryDelay):
			}
		}
	}
}

func (s *ProjectionService) handle(
	ctx context.Context,
	message kafka.Message,
) error {
	var envelope core_outbox.Envelope

	if err := json.Unmarshal(message.Value, &envelope); err != nil {
		// Сообщение невозможно разобрать: логируем и пропускаем, иначе
		// оно заблокирует партицию навсегда.
		s.log.Error(
			"skip malformed kafka message",
			zap.String("topic", message.Topic),
			zap.Int64("offset", message.Offset),
			zap.Error(err),
		)

		return nil
	}

	log := s.log.With(
		zap.String("aggregate_type", string(envelope.AggregateType)),
		zap.String("aggregate_id", envelope.AggregateID),
		zap.String("event_type", string(envelope.EventType)),
		zap.Int("version", envelope.Version),
	)

	collection, err := collectionFor(envelope.AggregateType)
	if err != nil {
		log.Error("skip event with unknown aggregate type", zap.Error(err))

		return nil
	}

	if envelope.EventType == core_outbox.EventTypeDeleted {
		var payload core_outbox.DeletedPayload

		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			log.Error("skip event with malformed payload", zap.Error(err))

			return nil
		}

		if err := s.repository.Delete(
			ctx,
			collection,
			payload.ID,
			payload.Version,
		); err != nil {
			return fmt.Errorf("delete projection: %w", err)
		}

		log.Debug("projection deleted")

		return nil
	}

	document, id, version, err := documentFor(envelope)
	if err != nil {
		log.Error("skip event with malformed payload", zap.Error(err))

		return nil
	}

	if err := s.repository.Upsert(
		ctx,
		collection,
		id,
		version,
		document,
	); err != nil {
		return fmt.Errorf("upsert projection: %w", err)
	}

	log.Debug("projection upserted")

	return nil
}

func collectionFor(aggregateType core_outbox.AggregateType) (string, error) {
	switch aggregateType {
	case core_outbox.AggregateTypeUser:
		return projection_mongo_repository.CollectionUsers, nil
	case core_outbox.AggregateTypeTask:
		return projection_mongo_repository.CollectionTasks, nil
	default:
		return "", fmt.Errorf("unknown aggregate type: %q", aggregateType)
	}
}

// documentFor разбирает payload в типизированную структуру, а затем
// раскладывает её в bson.M — так расхождение с контрактом события
// обнаруживается здесь, а не превращается в мусор в Mongo.
func documentFor(
	envelope core_outbox.Envelope,
) (bson.M, int, int, error) {
	switch envelope.AggregateType {
	case core_outbox.AggregateTypeUser:
		var payload core_outbox.UserPayload

		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			return nil, 0, 0, fmt.Errorf("unmarshal user payload: %w", err)
		}

		document := bson.M{
			"full_name":    payload.FullName,
			"phone_number": payload.PhoneNumber,
		}

		return document, payload.ID, payload.Version, nil
	case core_outbox.AggregateTypeTask:
		var payload core_outbox.TaskPayload

		if err := json.Unmarshal(envelope.Payload, &payload); err != nil {
			return nil, 0, 0, fmt.Errorf("unmarshal task payload: %w", err)
		}

		document := bson.M{
			"title":          payload.Title,
			"description":    payload.Description,
			"completed":      payload.Completed,
			"created_at":     payload.CreatedAt,
			"completed_at":   payload.CompletedAt,
			"author_user_id": payload.AuthorUserID,
		}

		return document, payload.ID, payload.Version, nil
	default:
		return nil, 0, 0, fmt.Errorf(
			"unknown aggregate type: %q",
			envelope.AggregateType,
		)
	}
}

func (s *ProjectionService) Close() error {
	if err := s.reader.Close(); err != nil {
		return fmt.Errorf("close kafka reader: %w", err)
	}

	return nil
}
