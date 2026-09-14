package integration

import (
	"encoding/json"
	"net/http"

	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	tasks_transport_http "github.com/1C-L0V3R/golang-td-app/internal/features/tasks/transport/http"
	users_transport_http "github.com/1C-L0V3R/golang-td-app/internal/features/users/transport/http"
)

// TestCreateWritesRowAndOutboxEventAtomically — базовая гарантия паттерна:
// один POST даёт ровно одну доменную строку и ровно одно событие 'created',
// payload которого совпадает с тем, что легло в Postgres. Проверяется для
// обоих агрегатов, потому что user и task пишутся разными репозиториями.
func (s *WriteSideSuite) TestCreateWritesRowAndOutboxEventAtomically() {
	user := s.createUser("Иван Пятаков", ptr("+79998887766"))

	s.Require().Equal(1, user.Version, "new user starts at version 1")

	userInDB, found := s.selectUser(user.ID)
	s.Require().True(found, "user row must exist in postgres")
	s.Equal(user.FullName, userInDB.FullName)
	s.Equal(user.Version, userInDB.Version)

	userEvents := s.outboxEvents(core_outbox.AggregateTypeUser, user.ID)
	s.Require().Len(userEvents, 1, "exactly one outbox event per create")

	userEvent := userEvents[0]
	s.Equal(core_outbox.EventTypeCreated, userEvent.EventType)
	s.Equal(core_outbox.AggregateTypeUser, userEvent.AggregateType)
	s.Equal(user.Version, userEvent.Version, "event version matches row version")
	s.requireUnpublished(userEvent)

	userPayload := s.userPayload(userEvent)
	s.Equal(core_outbox.UserPayload{
		ID:          user.ID,
		Version:     user.Version,
		FullName:    "Иван Пятаков",
		PhoneNumber: ptr("+79998887766"),
	}, userPayload)

	task := s.createTask("Домашнее задание", ptr("Сделать дз до четверга"), user.ID)

	s.Require().Equal(1, task.Version, "new task starts at version 1")
	s.False(task.Completed, "new task is not completed")
	s.Nil(task.CompletedAt)

	taskInDB, found := s.selectTask(task.ID)
	s.Require().True(found, "task row must exist in postgres")
	s.Equal(task.Title, taskInDB.Title)

	taskEvents := s.outboxEvents(core_outbox.AggregateTypeTask, task.ID)
	s.Require().Len(taskEvents, 1, "exactly one outbox event per create")

	taskEvent := taskEvents[0]
	s.Equal(core_outbox.EventTypeCreated, taskEvent.EventType)
	s.Equal(core_outbox.AggregateTypeTask, taskEvent.AggregateType)
	s.Equal(task.Version, taskEvent.Version)
	s.requireUnpublished(taskEvent)

	taskPayload := s.taskPayload(taskEvent)
	s.Equal(task.ID, taskPayload.ID)
	s.Equal("Домашнее задание", taskPayload.Title)
	s.Equal(ptr("Сделать дз до четверга"), taskPayload.Description)
	s.False(taskPayload.Completed)
	s.Nil(taskPayload.CompletedAt)
	s.Equal(user.ID, taskPayload.AuthorUserID)

	s.Equal("1", userEvent.AggregateID)
	s.Equal("1", taskEvent.AggregateID)

	s.Equal(2, s.outboxCount(), "no stray events were written")
}

// TestPatchIncrementsVersionAndEmitsUpdatedEvent — PATCH обязан двигать version
// вперёд и публиковать НОВУЮ версию
func (s *WriteSideSuite) TestPatchIncrementsVersionAndEmitsUpdatedEvent() {
	user := s.createUser("Иван Пятаков", ptr("+79998887766"))

	status, body := s.do(
		http.MethodPatch,
		"/api/v1/users/"+itoa(user.ID),
		map[string]any{
			"full_name":    "Мамут Рахал",
			"phone_number": nil,
		},
	)
	s.Require().Equal(http.StatusOK, status, "patch user: %s", body)

	var patchedUser users_transport_http.UserDTOResponse
	s.decode(body, &patchedUser)

	s.Equal(2, patchedUser.Version, "patch bumps version")
	s.Equal("Мамут Рахал", patchedUser.FullName)
	s.Nil(patchedUser.PhoneNumber, "explicit null clears the column")

	userInDB, found := s.selectUser(user.ID)
	s.Require().True(found)
	s.Equal(2, userInDB.Version)
	s.Nil(userInDB.PhoneNumber)

	userEvents := s.outboxEvents(core_outbox.AggregateTypeUser, user.ID)
	s.Require().Len(userEvents, 2, "created + updated")

	updatedUserEvent := userEvents[1]
	s.Equal(core_outbox.EventTypeUpdated, updatedUserEvent.EventType)
	s.Equal(2, updatedUserEvent.Version, "event carries the NEW version")
	s.Greater(updatedUserEvent.Seq, userEvents[0].Seq, "seq preserves write order")
	s.requireUnpublished(updatedUserEvent)

	updatedPayload := s.userPayload(updatedUserEvent)
	s.Equal("Мамут Рахал", updatedPayload.FullName)
	s.Nil(updatedPayload.PhoneNumber)
	s.Equal(2, updatedPayload.Version)

	task := s.createTask("Погулять с собакой", nil, user.ID)

	status, body = s.do(
		http.MethodPatch,
		"/api/v1/tasks/"+itoa(task.ID),
		map[string]any{"completed": true},
	)
	s.Require().Equal(http.StatusOK, status, "patch task: %s", body)

	var patchedTask tasks_transport_http.TaskDTOResponse
	s.decode(body, &patchedTask)

	s.Equal(2, patchedTask.Version)
	s.True(patchedTask.Completed)
	s.Require().NotNil(patchedTask.CompletedAt)

	taskEvents := s.outboxEvents(core_outbox.AggregateTypeTask, task.ID)
	s.Require().Len(taskEvents, 2, "created + updated")

	updatedTaskEvent := taskEvents[1]
	s.Equal(core_outbox.EventTypeUpdated, updatedTaskEvent.EventType)
	s.Equal(2, updatedTaskEvent.Version)
	s.requireUnpublished(updatedTaskEvent)

	updatedTaskPayload := s.taskPayload(updatedTaskEvent)
	s.True(updatedTaskPayload.Completed)
	s.Require().NotNil(updatedTaskPayload.CompletedAt, "completed_at must reach the projection")
	s.False(updatedTaskPayload.CompletedAt.Before(updatedTaskPayload.CreatedAt))
}

// TestDeleteEmitsDeletedEventWithVersion — DELETE удаляет строку и оставляет
// событие `deleted` с версией, под которой удаление произошло
func (s *WriteSideSuite) TestDeleteEmitsDeletedEventWithVersion() {
	user := s.createUser("Иван Пятаков", nil)
	task := s.createTask("Домашка", nil, user.ID)

	status, body := s.do(
		http.MethodPatch,
		"/api/v1/tasks/"+itoa(task.ID),
		map[string]any{"title": "Домашка по вышмату"},
	)
	s.Require().Equal(http.StatusOK, status, "patch task: %s", body)

	status, body = s.do(http.MethodDelete, "/api/v1/tasks/"+itoa(task.ID), nil)
	s.Require().Equal(http.StatusNoContent, status, "delete task: %s", body)
	s.Empty(body, "204 has no body")

	_, found := s.selectTask(task.ID)
	s.False(found, "task row must be gone from postgres")

	taskEvents := s.outboxEvents(core_outbox.AggregateTypeTask, task.ID)
	s.Require().Len(taskEvents, 3, "created + updated + deleted")

	deletedTaskEvent := taskEvents[2]
	s.Equal(core_outbox.EventTypeDeleted, deletedTaskEvent.EventType)
	s.Equal(2, deletedTaskEvent.Version, "deleted event carries the pre-delete version")
	s.requireUnpublished(deletedTaskEvent)

	var deletedPayload core_outbox.DeletedPayload
	s.Require().NoError(json.Unmarshal(deletedTaskEvent.Payload, &deletedPayload))
	s.Equal(core_outbox.DeletedPayload{ID: task.ID, Version: 2}, deletedPayload)

	s.Equal(4, s.outboxCount(), "delete does not erase the aggregate's event history")

	status, body = s.do(http.MethodDelete, "/api/v1/users/"+itoa(user.ID), nil)
	s.Require().Equal(http.StatusNoContent, status, "delete user: %s", body)

	_, found = s.selectUser(user.ID)
	s.False(found, "user row must be gone from postgres")

	userEvents := s.outboxEvents(core_outbox.AggregateTypeUser, user.ID)
	s.Require().Len(userEvents, 2, "created + deleted")
	s.Equal(core_outbox.EventTypeDeleted, userEvents[1].EventType)
	s.Equal(1, userEvents[1].Version)
}

// TestFailedWriteLeavesNoOutboxEvent
// если доменная запись не удалась, событие тоже не должно появиться. Иначе проектор
// применит изменение, которого в Postgres нет.
func (s *WriteSideSuite) TestFailedWriteLeavesNoOutboxEvent() {
	const missingID = 99999

	user := s.createUser("Иван Пятаков", nil)

	countBefore := s.outboxCount()
	s.Require().Equal(1, countBefore)

	status, body := s.do(http.MethodDelete, "/api/v1/users/"+itoa(missingID), nil)
	s.Require().Equal(http.StatusNotFound, status, "delete missing user: %s", body)

	status, body = s.do(http.MethodDelete, "/api/v1/tasks/"+itoa(missingID), nil)
	s.Require().Equal(http.StatusNotFound, status, "delete missing task: %s", body)

	status, body = s.do(
		http.MethodPatch,
		"/api/v1/users/"+itoa(missingID),
		map[string]any{"full_name": "Никого Нет"},
	)
	s.Require().Equal(http.StatusNotFound, status, "patch missing user: %s", body)

	status, body = s.do(http.MethodPost, "/api/v1/tasks", map[string]any{
		"title":          "Задача сироты",
		"author_user_id": missingID,
	})
	s.Require().Equal(http.StatusNotFound, status, "create task with missing author: %s", body)

	status, body = s.do(http.MethodPost, "/api/v1/users", map[string]any{
		"full_name": "ab",
	})
	s.Require().Equal(http.StatusBadRequest, status, "create user with short name: %s", body)

	s.Equal(countBefore, s.outboxCount(), "failed writes must not leak outbox events")

	task := s.createTask("Живая задача", nil, user.ID)

	taskEvents := s.outboxEvents(core_outbox.AggregateTypeTask, task.ID)
	s.Require().Len(taskEvents, 1)
	s.Equal(core_outbox.EventTypeCreated, taskEvents[0].EventType)
	s.Equal(countBefore+1, s.outboxCount())
}
