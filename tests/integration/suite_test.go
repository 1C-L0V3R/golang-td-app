package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	core_outbox_repository "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/outbox"
	core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"
	core_pgx_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool/pgx"
	core_http_middleware "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/middleware"
	core_http_server "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/server"
	tasks_postgres_repository "github.com/1C-L0V3R/golang-td-app/internal/features/tasks/repository/postgres"
	tasks_service "github.com/1C-L0V3R/golang-td-app/internal/features/tasks/service"
	tasks_transport_http "github.com/1C-L0V3R/golang-td-app/internal/features/tasks/transport/http"
	users_postgres_repository "github.com/1C-L0V3R/golang-td-app/internal/features/users/repository/postgres"
	users_service "github.com/1C-L0V3R/golang-td-app/internal/features/users/service"
	users_transport_http "github.com/1C-L0V3R/golang-td-app/internal/features/users/transport/http"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
)

const (
	postgresImage    = "postgres:18.4-bookworm"
	postgresUser     = "todoapp"
	postgresPassword = "todoapp"
	postgresDB       = "todoapp"

	migrationInit   = "000001_init.up.sql"
	migrationOutbox = "000002_outbox.up.sql"
)

type WriteSideSuite struct {
	suite.Suite

	container *postgres.PostgresContainer
	pool      *core_pgx_pool.Pool
	logger    *core_logger.Logger
	server    *httptest.Server
}

func TestWriteSideSuite(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test requires Docker: skipped in -short mode")
	}

	suite.Run(t, new(WriteSideSuite))
}

func (s *WriteSideSuite) SetupSuite() {
	ctx := context.Background()

	initScripts, err := migrationPaths(migrationInit, migrationOutbox)
	s.Require().NoError(err, "resolve migration paths")

	s.container, err = postgres.Run(
		ctx,
		postgresImage,
		postgres.WithDatabase(postgresDB),
		postgres.WithUsername(postgresUser),
		postgres.WithPassword(postgresPassword),
		postgres.WithInitScripts(initScripts...),
		postgres.BasicWaitStrategies(),
	)
	s.Require().NoError(err, "start postgres container")

	host, err := s.container.Host(ctx)
	s.Require().NoError(err, "get container host")

	port, err := s.container.MappedPort(ctx, "5432/tcp")
	s.Require().NoError(err, "get container mapped port")

	s.pool, err = core_pgx_pool.NewPool(ctx, core_pgx_pool.Config{
		Host:     host,
		Port:     port.Port(),
		User:     postgresUser,
		Password: postgresPassword,
		Database: postgresDB,
		Timeout:  10 * time.Second,
	})
	s.Require().NoError(err, "init postgres connection pool")

	s.logger, err = core_logger.NewLogger(core_logger.Config{
		Level:  "ERROR",
		Folder: s.T().TempDir(),
	})
	s.Require().NoError(err, "init logger")

	s.server = httptest.NewServer(s.newHandler())
}

func (s *WriteSideSuite) TearDownSuite() {
	if s.server != nil {
		s.server.Close()
	}

	if s.logger != nil {
		s.logger.Close()
	}

	if s.pool != nil {
		s.pool.Close()
	}

	if s.container != nil {
		s.Require().NoError(s.container.Terminate(context.Background()))
	}
}

func (s *WriteSideSuite) SetupTest() {
	ctx := context.Background()

	statements := []string{
		"TRUNCATE todoapp.tasks RESTART IDENTITY CASCADE;",
		"TRUNCATE todoapp.users RESTART IDENTITY CASCADE;",
		"TRUNCATE todoapp.outbox RESTART IDENTITY;",
	}

	for _, statement := range statements {
		_, err := s.pool.Exec(ctx, statement)
		s.Require().NoError(err, "truncate: %s", statement)
	}
}

func (s *WriteSideSuite) newHandler() http.Handler {
	outboxRepository := core_outbox_repository.NewOutboxRepository()
	tasksRepository := tasks_postgres_repository.NewTasksRepository(s.pool, outboxRepository)

	return s.newHandlerWithTasksRepository(tasksRepository)
}

func (s *WriteSideSuite) newHandlerWithTasksRepository(
	tasksRepository tasks_service.TasksRepository,
) http.Handler {
	outboxRepository := core_outbox_repository.NewOutboxRepository()

	usersRepository := users_postgres_repository.NewUsersRepository(s.pool, outboxRepository)
	usersHandler := users_transport_http.NewUsersHTTPHandler(
		users_service.NewUsersService(usersRepository),
	)

	tasksHandler := tasks_transport_http.NewTasksHTTPHandler(
		tasks_service.NewTasksService(tasksRepository),
	)

	apiRouter := core_http_server.NewAPIVersionRouter(core_http_server.ApiVersion1)
	apiRouter.RegisterRoutes(usersHandler.Routes()...)
	apiRouter.RegisterRoutes(tasksHandler.Routes()...)

	mux := http.NewServeMux()
	mux.Handle("/api/v1/", http.StripPrefix("/api/v1", apiRouter.WithMiddleware()))

	return core_http_middleware.ChainMiddleware(
		mux,
		core_http_middleware.CORS(),
		core_http_middleware.RequestID(),
		core_http_middleware.Logger(s.logger),
		core_http_middleware.Trace(),
		core_http_middleware.Panic(),
	)
}

func migrationPaths(names ...string) ([]string, error) {
	paths := make([]string, 0, len(names))

	for _, name := range names {
		path, err := filepath.Abs(filepath.Join("..", "..", "migrations", name))
		if err != nil {
			return nil, fmt.Errorf("abs path for %q: %w", name, err)
		}

		paths = append(paths, path)
	}

	return paths, nil
}

func (s *WriteSideSuite) do(method, path string, body any) (int, []byte) {
	s.T().Helper()

	var reader io.Reader

	if body != nil {
		raw, err := json.Marshal(body)
		s.Require().NoError(err, "marshal request body")

		reader = bytes.NewReader(raw)
	}

	request, err := http.NewRequest(method, s.server.URL+path, reader)
	s.Require().NoError(err, "build request")

	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := s.server.Client().Do(request)
	s.Require().NoError(err, "perform request")
	defer func() { _ = response.Body.Close() }()

	responseBody, err := io.ReadAll(response.Body)
	s.Require().NoError(err, "read response body")

	return response.StatusCode, responseBody
}

func (s *WriteSideSuite) decode(body []byte, target any) {
	s.T().Helper()

	s.Require().NoError(json.Unmarshal(body, target), "decode response: %s", body)
}

func (s *WriteSideSuite) createUser(fullName string, phoneNumber *string) users_transport_http.UserDTOResponse {
	s.T().Helper()

	status, body := s.do(http.MethodPost, "/api/v1/users", map[string]any{
		"full_name":    fullName,
		"phone_number": phoneNumber,
	})
	s.Require().Equal(http.StatusCreated, status, "create user: %s", body)

	var user users_transport_http.UserDTOResponse
	s.decode(body, &user)

	return user
}

func (s *WriteSideSuite) createTask(title string, description *string, authorUserID int) tasks_transport_http.TaskDTOResponse {
	s.T().Helper()

	status, body := s.do(http.MethodPost, "/api/v1/tasks", map[string]any{
		"title":          title,
		"description":    description,
		"author_user_id": authorUserID,
	})
	s.Require().Equal(http.StatusCreated, status, "create task: %s", body)

	var task tasks_transport_http.TaskDTOResponse
	s.decode(body, &task)

	return task
}

type outboxRow struct {
	Seq           int64
	ID            uuid.UUID
	AggregateType core_outbox.AggregateType
	AggregateID   string
	EventType     core_outbox.EventType
	Version       int
	Payload       []byte
	PublishedAt   *time.Time
	Attempts      int
}

func (s *WriteSideSuite) outboxEvents(
	aggregateType core_outbox.AggregateType,
	aggregateID int,
) []outboxRow {
	s.T().Helper()

	query := `
	SELECT seq, id, aggregate_type, aggregate_id, event_type, version, payload, published_at, attempts
	FROM todoapp.outbox
	WHERE aggregate_type=$1 AND aggregate_id=$2
	ORDER BY seq;
	`

	rows, err := s.pool.Query(
		context.Background(),
		query,
		string(aggregateType),
		fmt.Sprintf("%d", aggregateID),
	)
	s.Require().NoError(err, "query outbox events")
	defer rows.Close()

	events := make([]outboxRow, 0)

	for rows.Next() {
		var (
			event        outboxRow
			rawAggregate string
			rawEventType string
		)

		err := rows.Scan(
			&event.Seq,
			&event.ID,
			&rawAggregate,
			&event.AggregateID,
			&rawEventType,
			&event.Version,
			&event.Payload,
			&event.PublishedAt,
			&event.Attempts,
		)
		s.Require().NoError(err, "scan outbox event")

		event.AggregateType = core_outbox.AggregateType(rawAggregate)
		event.EventType = core_outbox.EventType(rawEventType)

		events = append(events, event)
	}

	s.Require().NoError(rows.Err(), "iterate outbox events")

	return events
}

func (s *WriteSideSuite) outboxCount() int {
	s.T().Helper()

	var count int

	err := s.pool.
		QueryRow(context.Background(), "SELECT count(*) FROM todoapp.outbox;").
		Scan(&count)
	s.Require().NoError(err, "count outbox rows")

	return count
}

type userRow struct {
	ID          int
	Version     int
	FullName    string
	PhoneNumber *string
}

func (s *WriteSideSuite) selectUser(id int) (userRow, bool) {
	s.T().Helper()

	var user userRow

	err := s.pool.QueryRow(
		context.Background(),
		"SELECT id, version, full_name, phone_number FROM todoapp.users WHERE id=$1;",
		id,
	).Scan(&user.ID, &user.Version, &user.FullName, &user.PhoneNumber)
	if err != nil {
		s.Require().ErrorIs(err, core_postgres_pool.ErrNoRows, "select user")

		return userRow{}, false
	}

	return user, true
}

type taskRow struct {
	ID          int
	Version     int
	Title       string
	Description *string
	Completed   bool
	CreatedAt   time.Time
	CompletedAt *time.Time
}

func (s *WriteSideSuite) selectTask(id int) (taskRow, bool) {
	s.T().Helper()

	var task taskRow

	err := s.pool.QueryRow(
		context.Background(),
		`SELECT id, version, title, description, completed, created_at, completed_at
		 FROM todoapp.tasks WHERE id=$1;`,
		id,
	).Scan(
		&task.ID,
		&task.Version,
		&task.Title,
		&task.Description,
		&task.Completed,
		&task.CreatedAt,
		&task.CompletedAt,
	)
	if err != nil {
		s.Require().ErrorIs(err, core_postgres_pool.ErrNoRows, "select task")

		return taskRow{}, false
	}

	return task, true
}

func (s *WriteSideSuite) requireUnpublished(event outboxRow) {
	s.T().Helper()

	require.Nil(s.T(), event.PublishedAt, "event seq=%d must stay unpublished", event.Seq)
	require.Zero(s.T(), event.Attempts, "event seq=%d must have no publish attempts", event.Seq)
	require.NotEqual(s.T(), uuid.Nil, event.ID, "event seq=%d must carry an id", event.Seq)
}

func (s *WriteSideSuite) userPayload(event outboxRow) core_outbox.UserPayload {
	s.T().Helper()

	var payload core_outbox.UserPayload
	s.Require().NoError(json.Unmarshal(event.Payload, &payload), "unmarshal user payload")

	return payload
}

func (s *WriteSideSuite) taskPayload(event outboxRow) core_outbox.TaskPayload {
	s.T().Helper()

	var payload core_outbox.TaskPayload
	s.Require().NoError(json.Unmarshal(event.Payload, &payload), "unmarshal task payload")

	return payload
}

func ptr[T any](value T) *T {
	return &value
}

func itoa(value int) string {
	return strconv.Itoa(value)
}
