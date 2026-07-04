package tasks_transport_http

import (
	"net/http"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_http_request "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/request"
	core_http_response "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/response"
)

type CreateTaskRequest struct {
	Title        string  `json:"title"           validate:"required,min=1,max=100"   example:"Домашнее задание"`
	Description  *string `json:"description"     validate:"omitempty,min=1,max=1000" example:"Сделать дз до четверга"`
	AuthorUserID int     `json:"author_user_id"  validate:"required" example:"5"`
}

type CreateTaskResponse TaskDTOResponse

// CreateTask 				godoc
// @Summary 				Create a Task
// @Description 			Create a New Task in system
// @Tags 					tasks
// @Accept 					json
// @Produce 				json
// @Param 					request body 		CreateTaskRequest 	true 			"Create Task Request Body"
// @Success 				201 	{object} 	CreateTaskResponse 					"Task was successfully created"
// @Failure 				400 	{object} 	core_http_response.ErrorResponse 	"Bad Request"
// @Failure 				404 	{object} 	core_http_response.ErrorResponse 	"Author was not found"
// @Failure 				500 	{object} 	core_http_response.ErrorResponse	"Internal Server Error"
// @Router 					/tasks [post]
func (h *TasksHTTPHandler) CreateTask(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_http_response.NewHTTPResponseHadler(log, rw)

	var request CreateTaskRequest
	if err := core_http_request.DecodeAndValidateRequest(r, &request); err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to decode and validate HTTP request",
		)

		return
	}

	taskDomain := domain.NewTaskUninitialized(
		request.Title,
		request.Description,
		request.AuthorUserID,
	)

	taskDomain, err := h.tasksService.CreateTask(ctx, taskDomain)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to create task",
		)

		return
	}

	response := CreateTaskResponse(taskDTOfromDomain(taskDomain))

	responseHandler.JSONResponse(response, http.StatusCreated)
}
