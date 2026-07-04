package tasks_transport_http

import (
	"net/http"

	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_http_request "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/request"
	core_http_response "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/response"
)

type GetTaskResponse TaskDTOResponse

// GetTask 			godoc
// @Summary 		Get Task
// @Descriprion 	Getting Task by their ID
// @Tags 			tasks
// @Produce 		json
// @Param 			id 	 path 	 int 	true 								"ID of the received Task"
// @Success 		200 	{object} 	GetTaskResponse 					"Task was found successfully"
// @Failure 		400 	{object} 	core_http_response.ErrorResponse 	"Bad Request"
// @Failure 		404 	{object} 	core_http_response.ErrorResponse 	"Task was not found"
// @Failure 		500 	{object} 	core_http_response.ErrorResponse 	"Internal Server Error"
// @Router 			/tasks/{id} [get]
func (h *TasksHTTPHandler) GetTask(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_http_response.NewHTTPResponseHadler(log, rw)

	taskID, err := core_http_request.GetIntPathValue(r, "id")
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get taskID path value",
		)

		return
	}

	taskDomain, err := h.tasksService.GetTask(ctx, taskID)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get task",
		)

		return
	}

	response := GetTaskResponse(taskDTOfromDomain(taskDomain))

	responseHandler.JSONResponse(response, http.StatusOK)
}
