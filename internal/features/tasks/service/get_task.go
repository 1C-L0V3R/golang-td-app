package tasks_service

import (
	"context"
	"fmt"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
)

func (s *TasksService) GetTask(
	ctx context.Context,
	id int,
) (domain.Task, error) {
	task, err := s.tasksRepository.GetTask(ctx, id)
	if err != nil {
		return domain.Task{}, fmt.Errorf("get task form respository: %w", err)
	}

	return task, nil
}
