package core_outbox

import (
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
	"github.com/google/uuid"
)

type AggregateType string

const (
	AggregateTypeUser AggregateType = "user"
	AggregateTypeTask AggregateType = "task"
)

type EventType string

const (
	EventTypeCreated  EventType = "created"
	EventTypeUpdated  EventType = "updated"
	EventTypeDeleted  EventType = "deleted"
	EventTypeSnapshot EventType = "snapshot"
)

// Event — запись outbox-таблицы. Пишется в той же транзакции, что и
// изменение агрегата, поэтому не может потеряться при падении приложения.
type Event struct {
	Seq           int64
	ID            uuid.UUID
	AggregateType AggregateType
	AggregateID   string
	EventType     EventType
	Version       int
	Payload       json.RawMessage
	CreatedAt     time.Time
}

// Envelope — формат сообщения в Kafka. Ключ сообщения — AggregateID,
// поэтому все события одного агрегата попадают в одну партицию.
type Envelope struct {
	ID            uuid.UUID       `json:"id"`
	AggregateType AggregateType   `json:"aggregate_type"`
	AggregateID   string          `json:"aggregate_id"`
	EventType     EventType       `json:"event_type"`
	Version       int             `json:"version"`
	OccurredAt    time.Time       `json:"occurred_at"`
	Payload       json.RawMessage `json:"payload"`
}

func (e Event) Envelope() Envelope {
	return Envelope{
		ID:            e.ID,
		AggregateType: e.AggregateType,
		AggregateID:   e.AggregateID,
		EventType:     e.EventType,
		Version:       e.Version,
		OccurredAt:    e.CreatedAt,
		Payload:       e.Payload,
	}
}

type UserPayload struct {
	ID          int     `json:"id"           bson:"_id"`
	Version     int     `json:"version"      bson:"version"`
	FullName    string  `json:"full_name"    bson:"full_name"`
	PhoneNumber *string `json:"phone_number" bson:"phone_number"`
}

type TaskPayload struct {
	ID           int        `json:"id"             bson:"_id"`
	Version      int        `json:"version"        bson:"version"`
	Title        string     `json:"title"          bson:"title"`
	Description  *string    `json:"description"    bson:"description"`
	Completed    bool       `json:"completed"      bson:"completed"`
	CreatedAt    time.Time  `json:"created_at"     bson:"created_at"`
	CompletedAt  *time.Time `json:"completed_at"   bson:"completed_at"`
	AuthorUserID int        `json:"author_user_id" bson:"author_user_id"`
}

// DeletedPayload — для события удаления тело агрегата уже недоступно,
// нужны только идентификатор и версия, под которой удаление произошло.
type DeletedPayload struct {
	ID      int `json:"id"      bson:"_id"`
	Version int `json:"version" bson:"version"`
}

func newEvent(
	aggregateType AggregateType,
	aggregateID int,
	eventType EventType,
	version int,
	payload any,
) (Event, error) {
	rawPayload, err := json.Marshal(payload)
	if err != nil {
		return Event{}, fmt.Errorf("marshal %s payload: %w", aggregateType, err)
	}

	return Event{
		ID:            uuid.New(),
		AggregateType: aggregateType,
		AggregateID:   strconv.Itoa(aggregateID),
		EventType:     eventType,
		Version:       version,
		Payload:       rawPayload,
		CreatedAt:     time.Now(),
	}, nil
}

func NewUserEvent(eventType EventType, user domain.User) (Event, error) {
	return newEvent(
		AggregateTypeUser,
		user.ID,
		eventType,
		user.Version,
		UserPayload{
			ID:          user.ID,
			Version:     user.Version,
			FullName:    user.FullName,
			PhoneNumber: user.PhoneNumber,
		},
	)
}

func NewTaskEvent(eventType EventType, task domain.Task) (Event, error) {
	return newEvent(
		AggregateTypeTask,
		task.ID,
		eventType,
		task.Version,
		TaskPayload{
			ID:           task.ID,
			Version:      task.Version,
			Title:        task.Title,
			Description:  task.Description,
			Completed:    task.Completed,
			CreatedAt:    task.CreatedAt,
			CompletedAt:  task.CompletedAt,
			AuthorUserID: task.AuthorUserID,
		},
	)
}

func NewDeletedEvent(
	aggregateType AggregateType,
	id int,
	version int,
) (Event, error) {
	return newEvent(
		aggregateType,
		id,
		EventTypeDeleted,
		version,
		DeletedPayload{
			ID:      id,
			Version: version,
		},
	)
}
