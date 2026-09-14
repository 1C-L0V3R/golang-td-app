package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	core_outbox_repository "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/outbox"
	core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"
	outbox_postgres_repository "github.com/1C-L0V3R/golang-td-app/internal/features/outbox/repository/postgres"
	tasks_postgres_repository "github.com/1C-L0V3R/golang-td-app/internal/features/tasks/repository/postgres"
	tasks_service "github.com/1C-L0V3R/golang-td-app/internal/features/tasks/service"
	tasks_transport_http "github.com/1C-L0V3R/golang-td-app/internal/features/tasks/transport/http"
)

const apiRequestTimeout = 15 * time.Second

// TestConcurrentPatchAllowsExactlyOneCommit проверяет optimistic locking через
// публичный HTTP API.
//
// Предусловия:
//   - в Postgres есть task с version=1;
//   - тестовый барьер заставляет все PATCH сначала прочитать одну и ту же
//     version=1, после чего одновременно передать изменения реальному
//     Postgres-репозиторию.
//
// Шаги выполнения:
//  1. Создать пользователя и task.
//  2. Отправить восемь параллельных PATCH с разными title.
//  3. Получить агрегат через GET и прочитать события из outbox.
//
// Ожидаемый результат: один PATCH коммитится с 200, остальные получают 409;
// версия становится равна 2, а состояние совпадает с единственным победителем.
//
// Assertions: распределение HTTP-статусов 1x200/7x409, version=2 в API и БД,
// title равен ответу успешного PATCH, в outbox только created(v1)+updated(v2).
func (s *WriteSideSuite) TestConcurrentPatchAllowsExactlyOneCommit() {
	const parallelRequests = 8

	user := s.createUser("Конкурентный Автор", nil)
	task := s.createTask("Исходное состояние", ptr("version=1"), user.ID)
	s.Require().Equal(1, task.Version)

	realRepository := tasks_postgres_repository.NewTasksRepository(
		s.pool,
		core_outbox_repository.NewOutboxRepository(),
	)
	barrierRepository := newSameVersionBarrierRepository(
		realRepository,
		task.ID,
		parallelRequests,
	)

	server := httptest.NewServer(s.newHandlerWithTasksRepository(barrierRepository))
	defer server.Close()

	client := server.Client()
	client.Timeout = apiRequestTimeout

	type patchResult struct {
		title  string
		status int
		body   []byte
		err    error
	}

	start := make(chan struct{})
	results := make(chan patchResult, parallelRequests)

	for i := 0; i < parallelRequests; i++ {
		title := fmt.Sprintf("Победитель PATCH #%d", i)

		go func() {
			<-start

			status, body, err := performJSONRequest(
				client,
				server.URL,
				http.MethodPatch,
				"/api/v1/tasks/"+itoa(task.ID),
				map[string]any{"title": title},
			)
			results <- patchResult{title: title, status: status, body: body, err: err}
		}()
	}

	close(start)

	successes := 0
	conflicts := 0
	winnerTitle := ""

	for i := 0; i < parallelRequests; i++ {
		result := <-results
		s.Require().NoError(result.err)

		switch result.status {
		case http.StatusOK:
			successes++
			winnerTitle = result.title

			var response tasks_transport_http.TaskDTOResponse
			s.decode(result.body, &response)
			s.Equal(2, response.Version)
			s.Equal(result.title, response.Title)
		case http.StatusConflict:
			conflicts++
			s.Contains(string(result.body), "conflict")
		default:
			s.Failf("unexpected PATCH status", "status=%d body=%s", result.status, result.body)
		}
	}

	s.Equal(1, successes, "exactly one request may commit version=1")
	s.Equal(parallelRequests-1, conflicts, "all stale writers must receive 409")
	s.Require().NotEmpty(winnerTitle)

	status, body := s.do(http.MethodGet, "/api/v1/tasks/"+itoa(task.ID), nil)
	s.Require().Equal(http.StatusOK, status, "get task: %s", body)

	var stored tasks_transport_http.TaskDTOResponse
	s.decode(body, &stored)
	s.Equal(2, stored.Version, "aggregate version increments exactly once")
	s.Equal(winnerTitle, stored.Title, "stored state belongs to the only successful PATCH")

	dbTask, found := s.selectTask(task.ID)
	s.Require().True(found)
	s.Equal(2, dbTask.Version)
	s.Equal(winnerTitle, dbTask.Title)

	events := s.outboxEvents(core_outbox.AggregateTypeTask, task.ID)
	s.Require().Len(events, 2, "losing PATCH requests must not emit events")
	s.Equal([]int{1, 2}, []int{events[0].Version, events[1].Version})
	s.Equal(core_outbox.EventTypeUpdated, events[1].EventType)
	s.Equal(winnerTitle, s.taskPayload(events[1]).Title)
}

// TestExtremePayloadRoundTripsWithoutCorruption проверяет UTF-8 и длинные
// значения отдельно для POST и PATCH.
//
// Предусловия: существует автор; строки длиннее нескольких КБ в UTF-8, но не
// превышают контрактные ограничения 100/1000 Unicode-символов.
//
// Шаги выполнения:
//  1. POST task со спецсимволами, переводами строк, Unicode и эмодщи.
//  2. GET созданного task и побайтовое сравнение строковых полей.
//  3. PATCH новыми экстремальными строками, затем повторный GET.
//
// Ожидаемый результат: API отвечает 201/200/200/200, данные не нормализуются,
// не обрезаются и не повреждаются ни в Postgres, ни в outbox.
//
// Assertions: byte slices title/description после POST, PATCH и GET идентичны
// отправленным; PATCH увеличивает version ровно до 2; payload outbox совпадает.
func (s *WriteSideSuite) TestExtremePayloadRoundTripsWithoutCorruption() {
	user := s.createUser("Unicode Автор", nil)

	postTitle := "POST: кавычки \"'\", строка\n№2, Unicode 漢字, emoji 🚀"
	postDescription := "POST\\path\t\"quoted\"\n" + strings.Repeat("🚀漢é", 300)
	s.Greater(len([]byte(postDescription)), 2*1024, "payload must be several KB in UTF-8")
	s.LessOrEqual(len([]rune(postDescription)), 1000, "payload must satisfy API contract")

	status, body := s.do(http.MethodPost, "/api/v1/tasks", map[string]any{
		"title":          postTitle,
		"description":    postDescription,
		"author_user_id": user.ID,
	})
	s.Require().Equal(http.StatusCreated, status, "create task: %s", body)

	var created tasks_transport_http.TaskDTOResponse
	s.decode(body, &created)
	s.Require().NotNil(created.Description)
	s.Equal([]byte(postTitle), []byte(created.Title))
	s.Equal([]byte(postDescription), []byte(*created.Description))

	status, body = s.do(http.MethodGet, "/api/v1/tasks/"+itoa(created.ID), nil)
	s.Require().Equal(http.StatusOK, status, "get created task: %s", body)

	var afterCreate tasks_transport_http.TaskDTOResponse
	s.decode(body, &afterCreate)
	s.Require().NotNil(afterCreate.Description)
	s.Equal([]byte(postTitle), []byte(afterCreate.Title), "GET must preserve title bytes")
	s.Equal([]byte(postDescription), []byte(*afterCreate.Description), "GET must preserve description bytes")

	patchTitle := "PATCH: []{}<> & \\ / \"'\nкириллица العربية 🧪"
	patchDescription := "PATCH\r\n\t\\\"'\n" + strings.Repeat("🧪雪界", 300)
	s.Greater(len([]byte(patchDescription)), 2*1024, "PATCH payload must be several KB in UTF-8")
	s.LessOrEqual(len([]rune(patchDescription)), 1000, "PATCH payload must satisfy API contract")

	status, body = s.do(http.MethodPatch, "/api/v1/tasks/"+itoa(created.ID), map[string]any{
		"title":       patchTitle,
		"description": patchDescription,
	})
	s.Require().Equal(http.StatusOK, status, "patch task: %s", body)

	var patched tasks_transport_http.TaskDTOResponse
	s.decode(body, &patched)
	s.Equal(2, patched.Version)
	s.Require().NotNil(patched.Description)
	s.Equal([]byte(patchTitle), []byte(patched.Title))
	s.Equal([]byte(patchDescription), []byte(*patched.Description))

	status, body = s.do(http.MethodGet, "/api/v1/tasks/"+itoa(created.ID), nil)
	s.Require().Equal(http.StatusOK, status, "get patched task: %s", body)

	var afterPatch tasks_transport_http.TaskDTOResponse
	s.decode(body, &afterPatch)
	s.Require().NotNil(afterPatch.Description)
	s.Equal(2, afterPatch.Version)
	s.Equal([]byte(patchTitle), []byte(afterPatch.Title), "GET must preserve patched title bytes")
	s.Equal([]byte(patchDescription), []byte(*afterPatch.Description), "GET must preserve patched description bytes")

	dbTask, found := s.selectTask(created.ID)
	s.Require().True(found)
	s.Require().NotNil(dbTask.Description)
	s.Equal([]byte(patchTitle), []byte(dbTask.Title))
	s.Equal([]byte(patchDescription), []byte(*dbTask.Description))

	events := s.outboxEvents(core_outbox.AggregateTypeTask, created.ID)
	s.Require().Len(events, 2)
	s.Equal([]byte(postDescription), []byte(*s.taskPayload(events[0]).Description))
	s.Equal([]byte(patchDescription), []byte(*s.taskPayload(events[1]).Description))
}

// TestOutboxUnderParallelAggregateLoad проверяет outbox под параллельной
// нагрузкой независимых агрегатов.
//
// Предусловия: один существующий автор; relay не запущен, поэтому все события
// должны оставаться доступными как unpublished.
//
// Шаги выполнения:
//  1. Параллельно запустить 24 workflow разных task-агрегатов.
//  2. В каждом workflow выполнить POST и три последовательных PATCH.
//  3. Проверить строки outbox и выбрать весь батч production-репозиторием relay.
//
// Ожидаемый результат: записаны все 97 событий (1 user + 24*(1+3) task), для
// каждого task порядок version равен 1,2,3,4, потерь и дубликатов UUID нет.
//
// Assertions: точное число строк и доступных relay событий, возрастающие seq и
// version внутри агрегата, корректные event type/payload, published_at=nil и
// attempts=0 для каждой task-записи.
func (s *WriteSideSuite) TestOutboxUnderParallelAggregateLoad() {
	const (
		aggregateCount      = 24
		updatesPerAggregate = 3
	)

	user := s.createUser("Автор Нагрузки", nil)
	client := s.server.Client()
	client.Timeout = apiRequestTimeout

	type workflowResult struct {
		taskID int
		titles []string
		err    error
	}

	results := make(chan workflowResult, aggregateCount)
	start := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(aggregateCount)

	for workerID := 0; workerID < aggregateCount; workerID++ {
		go func() {
			defer workers.Done()
			<-start

			titles := []string{fmt.Sprintf("load-%02d-v1", workerID)}
			status, body, err := performJSONRequest(
				client,
				s.server.URL,
				http.MethodPost,
				"/api/v1/tasks",
				map[string]any{
					"title":          titles[0],
					"description":    fmt.Sprintf("aggregate-%02d", workerID),
					"author_user_id": user.ID,
				},
			)
			if err != nil {
				results <- workflowResult{err: fmt.Errorf("worker %d POST: %w", workerID, err)}
				return
			}
			if status != http.StatusCreated {
				results <- workflowResult{err: fmt.Errorf("worker %d POST status=%d body=%s", workerID, status, body)}
				return
			}

			var task tasks_transport_http.TaskDTOResponse
			if err := json.Unmarshal(body, &task); err != nil {
				results <- workflowResult{err: fmt.Errorf("worker %d decode POST: %w", workerID, err)}
				return
			}

			for update := 1; update <= updatesPerAggregate; update++ {
				title := fmt.Sprintf("load-%02d-v%d", workerID, update+1)
				titles = append(titles, title)

				status, body, err = performJSONRequest(
					client,
					s.server.URL,
					http.MethodPatch,
					"/api/v1/tasks/"+itoa(task.ID),
					map[string]any{"title": title},
				)
				if err != nil {
					results <- workflowResult{err: fmt.Errorf("worker %d PATCH %d: %w", workerID, update, err)}
					return
				}
				if status != http.StatusOK {
					results <- workflowResult{err: fmt.Errorf("worker %d PATCH %d status=%d body=%s", workerID, update, status, body)}
					return
				}
			}

			results <- workflowResult{taskID: task.ID, titles: titles}
		}()
	}

	close(start)
	workers.Wait()
	close(results)

	workflows := make([]workflowResult, 0, aggregateCount)
	for result := range results {
		s.Require().NoError(result.err)
		workflows = append(workflows, result)
	}
	s.Require().Len(workflows, aggregateCount)

	expectedTaskEvents := aggregateCount * (1 + updatesPerAggregate)
	expectedTotalEvents := 1 + expectedTaskEvents
	s.Equal(expectedTotalEvents, s.outboxCount(), "no outbox event may be lost")

	seenEventIDs := make(map[string]struct{}, expectedTaskEvents)
	for _, workflow := range workflows {
		events := s.outboxEvents(core_outbox.AggregateTypeTask, workflow.taskID)
		s.Require().Len(events, 1+updatesPerAggregate)

		for index, event := range events {
			expectedVersion := index + 1
			s.Equal(expectedVersion, event.Version)
			s.Equal(itoa(workflow.taskID), event.AggregateID)
			s.requireUnpublished(event)

			if index == 0 {
				s.Equal(core_outbox.EventTypeCreated, event.EventType)
			} else {
				s.Equal(core_outbox.EventTypeUpdated, event.EventType)
				s.Greater(event.Seq, events[index-1].Seq, "seq must grow inside one aggregate")
			}

			payload := s.taskPayload(event)
			s.Equal(workflow.taskID, payload.ID)
			s.Equal(expectedVersion, payload.Version)
			s.Equal(workflow.titles[index], payload.Title)

			eventID := event.ID.String()
			_, duplicate := seenEventIDs[eventID]
			s.False(duplicate, "event UUID must be unique: %s", eventID)
			seenEventIDs[eventID] = struct{}{}
		}
	}
	s.Len(seenEventIDs, expectedTaskEvents)

	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	s.Require().NoError(err)
	defer func() { _ = tx.Rollback(ctx) }()

	relayRepository := outbox_postgres_repository.NewOutboxRepository(s.pool)
	available, err := relayRepository.FetchUnpublished(ctx, tx, expectedTotalEvents+1)
	s.Require().NoError(err)
	s.Require().Len(available, expectedTotalEvents, "every event must be available to relay")

	for index, event := range available {
		s.True(json.Valid(event.Payload), "event seq=%d must contain valid JSON", event.Seq)
		if index > 0 {
			s.Greater(event.Seq, available[index-1].Seq, "relay batch must be ordered by seq")
		}
	}
}

// TestOutboxFailureRollsBackDomainWrite покрывает сценарий:
// сбой второй половины transactional outbox после успешного INSERT агрегата.
//
// Предусловия: task-репозиторий использует реальную транзакцию и тестовый
// outbox writer, который всегда возвращает ошибку.
//
// Шаги выполнения: отправить POST /tasks, дождаться 500, проверить таблицы,
// затем выполнить обычный POST через исправный handler.
//
// Ожидаемый результат: неуспешный POST полностью откатывается; следующий POST
// успешно и атомарно создаёт task и событие.
//
// Assertions: HTTP 500, число task/outbox не изменилось после сбоя; после
// контрольного POST присутствуют ровно одна task и одно task-событие.
func (s *WriteSideSuite) TestOutboxFailureRollsBackDomainWrite() {
	user := s.createUser("Автор Транзакции", nil)
	outboxBefore := s.outboxCount()

	failingRepository := tasks_postgres_repository.NewTasksRepository(
		s.pool,
		failingOutboxRepository{err: errors.New("forced outbox insert failure")},
	)
	server := httptest.NewServer(s.newHandlerWithTasksRepository(failingRepository))
	defer server.Close()

	client := server.Client()
	client.Timeout = apiRequestTimeout

	status, body, err := performJSONRequest(
		client,
		server.URL,
		http.MethodPost,
		"/api/v1/tasks",
		map[string]any{
			"title":          "Не должна сохраниться",
			"description":    "domain INSERT succeeds before outbox INSERT fails",
			"author_user_id": user.ID,
		},
	)
	s.Require().NoError(err)
	s.Equal(http.StatusInternalServerError, status, "body=%s", body)
	s.Contains(string(body), "forced outbox insert failure")

	var taskCount int
	err = s.pool.QueryRow(context.Background(), "SELECT count(*) FROM todoapp.tasks;").Scan(&taskCount)
	s.Require().NoError(err)
	s.Zero(taskCount, "domain row must roll back when outbox insert fails")
	s.Equal(outboxBefore, s.outboxCount(), "failed transaction must not add an outbox row")

	created := s.createTask("Контрольная успешная запись", nil, user.ID)
	s.Equal(1, created.Version)

	events := s.outboxEvents(core_outbox.AggregateTypeTask, created.ID)
	s.Require().Len(events, 1)
	s.Equal(core_outbox.EventTypeCreated, events[0].EventType)
	s.Equal(outboxBefore+1, s.outboxCount())
}

type sameVersionBarrierRepository struct {
	tasks_service.TasksRepository

	targetID     int
	participants int32
	arrived      atomic.Int32
	release      chan struct{}
	releaseOnce  sync.Once
}

func newSameVersionBarrierRepository(
	repository tasks_service.TasksRepository,
	targetID int,
	participants int,
) *sameVersionBarrierRepository {
	return &sameVersionBarrierRepository{
		TasksRepository: repository,
		targetID:        targetID,
		participants:    int32(participants),
		release:         make(chan struct{}),
	}
}

func (r *sameVersionBarrierRepository) GetTask(ctx context.Context, id int) (domain.Task, error) {
	task, err := r.TasksRepository.GetTask(ctx, id)
	if err != nil || id != r.targetID {
		return task, err
	}

	if r.arrived.Add(1) == r.participants {
		r.releaseOnce.Do(func() { close(r.release) })
	}

	select {
	case <-r.release:
		return task, nil
	case <-ctx.Done():
		return domain.Task{}, ctx.Err()
	}
}

type failingOutboxRepository struct {
	err error
}

func (r failingOutboxRepository) InsertEvent(
	context.Context,
	core_postgres_pool.Querier,
	core_outbox.Event,
) error {
	return r.err
}

func performJSONRequest(
	client *http.Client,
	baseURL string,
	method string,
	path string,
	body any,
) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return 0, nil, fmt.Errorf("marshal request body: %w", err)
		}
		reader = bytes.NewReader(raw)
	}

	request, err := http.NewRequest(method, baseURL+path, reader)
	if err != nil {
		return 0, nil, fmt.Errorf("build request: %w", err)
	}
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.Do(request)
	if err != nil {
		return 0, nil, fmt.Errorf("perform request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	responseBody, err := io.ReadAll(response.Body)
	if err != nil {
		return 0, nil, fmt.Errorf("read response body: %w", err)
	}

	return response.StatusCode, responseBody, nil
}
