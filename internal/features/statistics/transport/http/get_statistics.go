package statistics_transport_http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_http_request "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/request"
	core_http_response "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/response"
)

type GetStatisticsResponse struct {
	TasksCreated           int      `json:"tasks_created"`
	TasksCompleted         int      `json:"tasks_completed"`
	TasksCompletedRate     *float64 `json:"tasks_completed_rate"`
	TasksAvgCompletionTime *string  `json:"tasks_avg_completion_time"`
}

func (h *StatisticsHTTPHandler) GetStatistics(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_http_response.NewHTTPResponseHadler(log, rw)

	queryParams, err := GetUserIDFromToQueryParams(r)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get userID/from/to query params",
		)

		return
	}

	statistics, err := h.statisticsService.GetStatistics(
		ctx,
		queryParams.userID,
		queryParams.from,
		queryParams.to,
	)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get statistics",
		)

		return
	}

	response := toDTOfromDomain(statistics)

	responseHandler.JSONResponse(response, http.StatusOK)
}

type queryParams struct {
	userID *int
	from   *time.Time
	to     *time.Time
}

func toDTOfromDomain(statistics domain.Statistics) GetStatisticsResponse {
	var avgTime *string
	if statistics.TasksAvgCompletionTime != nil {
		duration := statistics.TasksAvgCompletionTime.String()
		avgTime = &duration
	}

	return GetStatisticsResponse{
		TasksCreated:           statistics.TasksCreated,
		TasksCompleted:         statistics.TasksCompleted,
		TasksCompletedRate:     statistics.TasksCompletedRate,
		TasksAvgCompletionTime: avgTime,
	}
}

func GetUserIDFromToQueryParams(r *http.Request) (queryParams, error) {
	const (
		userIDQueryParamKey = "user_id"
		fromQueryParamKey   = "from"
		toQueryParamKey     = "to"
	)

	userID, err := core_http_request.GetIntQueryParam(r, userIDQueryParamKey)
	if err != nil {
		return queryParams{}, fmt.Errorf("get 'user_id' query param: %w", err)
	}

	from, err := core_http_request.GetDateQueryParam(r, fromQueryParamKey)
	if err != nil {
		return queryParams{}, fmt.Errorf("get 'from' query param: %w", err)
	}

	to, err := core_http_request.GetDateQueryParam(r, toQueryParamKey)
	if err != nil {
		return queryParams{}, fmt.Errorf("get 'to' query param: %w", err)
	}

	return queryParams{
		userID: userID,
		from:   from,
		to:     to,
	}, nil
}
