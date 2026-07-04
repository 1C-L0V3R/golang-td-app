package tasks_transport_http

import (
	"net/http"

	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_http_request "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/request"
	core_http_response "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/response"
)

// DeleteTask 			godoc
// @Summary 			Deleting Task
// @Description 		Delete existing in system Task by their ID
// @Tags 				tasks
// @Param 				id 	 path 	int 	true 								"Deletin task ID"
// @Success 			204 													"Task deleted successfully"
// @Failure 			400 	{object} 	core_http_response.ErrorResponse 	"Bad Request"
// @Failure 			404 	{object} 	core_http_response.ErrorResponse 	"Task was not found"
// @Failure 			500 	{object} 	core_http_response.ErrorResponse 	"Internal Server Error"
// @Router 				/tasks/{id} [delete]
func (h *TasksHTTPHandler) DeleteTask(rw http.ResponseWriter, r *http.Request) {
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

	if err := h.tasksService.DeleteTask(ctx, taskID); err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to delete task",
		)

		return
	}

	responseHandler.NoContentResponse()
}
