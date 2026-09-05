package tasks_postgres_repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
	core_errors "github.com/1C-L0V3R/golang-td-app/internal/core/errors"
	core_outbox "github.com/1C-L0V3R/golang-td-app/internal/core/outbox"
	core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"
)

func (r *TasksRepository) PatchTask(
	ctx context.Context,
	id int,
	task domain.Task,
) (domain.Task, error) {
	ctx, cancel := context.WithTimeout(ctx, r.pool.OpTimeout())
	defer cancel()

	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return domain.Task{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	query := `
	UPDATE todoapp.tasks
	SET
		title=$1,
		description=$2,
		completed=$3,
		completed_at=$4,
		version=version + 1
	WHERE id=$5 AND version=$6

	RETURNING
		id,
		version,
		title,
		description,
		completed,
		created_at,
		completed_at,
		author_user_id;
	`

	row := tx.QueryRow(
		ctx,
		query,
		task.Title,
		task.Description,
		task.Completed,
		task.CompletedAt,
		id,
		task.Version,
	)

	var taskModel TaskModel

	err = row.Scan(
		&taskModel.ID,
		&taskModel.Version,
		&taskModel.Title,
		&taskModel.Description,
		&taskModel.Completed,
		&taskModel.CreatedAt,
		&taskModel.CompletedAt,
		&taskModel.AuthorUserID,
	)
	if err != nil {
		if errors.Is(err, core_postgres_pool.ErrNoRows) {
			return domain.Task{}, fmt.Errorf(
				"task with id='%d' concurrently accessed: %w,",
				id,
				core_errors.ErrConflict,
			)
		}

		return domain.Task{}, fmt.Errorf("scan error: %w", err)
	}

	taskDomain := taskDomainFromModel(taskModel)

	event, err := core_outbox.NewTaskEvent(core_outbox.EventTypeUpdated, taskDomain)
	if err != nil {
		return domain.Task{}, fmt.Errorf("build outbox event: %w", err)
	}

	if err := r.outbox.InsertEvent(ctx, tx, event); err != nil {
		return domain.Task{}, fmt.Errorf("insert outbox event: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return domain.Task{}, fmt.Errorf("commit transaction: %w", err)
	}

	return taskDomain, nil
}
