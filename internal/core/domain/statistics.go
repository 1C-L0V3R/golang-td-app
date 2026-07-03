package domain

import "time"

type Statistics struct {
	TasksCreated           int
	TasksCompleted         int
	TasksCompletedRate     *float64
	TasksAvgCompletionTime *time.Duration
}

func NewStatistics(
	tasksCreated int,
	tasksCompleted int,
	tasksCompletedRate *float64,
	tasksAvgCompletionTime *time.Duration,
) Statistics {
	return Statistics{
		TasksCreated:           tasksCreated,
		TasksCompleted:         tasksCompleted,
		TasksCompletedRate:     tasksCompletedRate,
		TasksAvgCompletionTime: tasksAvgCompletionTime,
	}
}
