package tasks_transport_http

import (
	"fmt"
	"net/http"

	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_http_request "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/request"
	core_http_response "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/response"
)

type GetTasksResponse []TaskDTOResponse

// GetTasks 			godoc
// @Summary 			Get Task List
// @Description			Viewing the list of existing in the system Tasks, with optional pagination and/or filtering by authorID
// @Tags 				tasks
// @Produce 			json
// @Param 				user_id 	query 	int 	false 								"Filtering tasks by authorID"
// @Param 				limit 		query 	int 	false 								"Size of the Task Page"
// @Param 				offset 		query 	int 	false 								"Tasks Page offset"
// @Success 			200 		{object} 		GetTasksResponse 					"Task List"
// @Failure 			400 		{object} 		core_http_response.ErrorResponse 	"Bad Request"
// @Failure 			400 		{object} 		core_http_response.ErrorResponse 	"Internal Server Error"
// @Router 				/tasks [get]
func (h *TasksHTTPHandler) GetTasks(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_http_response.NewHTTPResponseHadler(log, rw)

	userID, limit, offset, err := GetUserIDLimitOffsetQueryParams(r)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get userID/limit/offset query params",
		)

		return
	}

	tasksDomains, err := h.tasksService.GetTasks(
		ctx,
		userID,
		limit,
		offset,
	)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get tasks",
		)

		return
	}

	response := GetTasksResponse(TaskDTOsFromDomains(tasksDomains))

	responseHandler.JSONResponse(response, http.StatusOK)

}

func GetUserIDLimitOffsetQueryParams(r *http.Request) (*int, *int, *int, error) {
	const (
		userIDQueryParamKey = "user_id"
		limitQueryParamKey  = "limit"
		offsetQueryParamKey = "offset"
	)

	userID, err := core_http_request.GetIntQueryParam(r, userIDQueryParamKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get 'user_id query param: %w", err)
	}

	limit, err := core_http_request.GetIntQueryParam(r, limitQueryParamKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get 'limit' query param: %w", err)
	}

	offset, err := core_http_request.GetIntQueryParam(r, offsetQueryParamKey)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get 'offset' query param: %w", err)
	}

	return userID, limit, offset, nil
}
