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
	TasksCreated           int      `json:"tasks_created"             example:"50"`
	TasksCompleted         int      `json:"tasks_completed"           example:"10"`
	TasksCompletedRate     *float64 `json:"tasks_completed_rate"      example:"20"`
	TasksAvgCompletionTime *string  `json:"tasks_avg_completion_time" example:"1h33m"`
}

// GetStatistics 			godoc
// @Summary 				Get Statistics
// @Description 			Retrieving task Statistics with optional filtering by user_id and/or time range
// @Tags 					statistics
// @Produce 				json
// @Param 					user_id 	query 	int 	 false 								"Filtering Statistics for a specific user"
// @Param 					from 		query 	string 	 false 								"Start of the Statistics review period (inclusive), format: YYYY-MM-DD"
// @Param 					to 			query 	string 	 false 								"End of the Statistics review period (exclusive), format: YYYY-MM-DD"
// @Success 				200 		{object} 		 GetStatisticsResponse 				"Collect Statistics Successfully"
// @Failure 				400 		{object} 		 core_http_response.ErrorResponse 	"Bad Request"
// @Failure 				500 		{object} 		 core_http_response.ErrorResponse 	"Internal Server Error"
// @Router 					/statistics [get]
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
