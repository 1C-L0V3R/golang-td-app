package tasks_transport_http

import (
	"fmt"
	"net/http"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_http_request "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/request"
	core_http_response "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/response"
	core_http_types "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/types"
)

type PatchTaskRequest struct {
	Title       core_http_types.Nullable[string] `json:"title"       swaggertype:"string"  example:"Погулять с собакой"`
	Description core_http_types.Nullable[string] `json:"description" swaggertype:"string"  example:"null"`
	Completed   core_http_types.Nullable[bool]   `json:"completed"   swaggertype:"boolean"`
}

type PatchTaskResponse TaskDTOResponse

func (r *PatchTaskRequest) Validate() error {
	if r.Title.Set {
		if r.Title.Value == nil {
			return fmt.Errorf("`Title` can't be NULL")
		}

		titleLen := len([]rune(*r.Title.Value))
		if titleLen < 1 || titleLen > 100 {
			return fmt.Errorf("`Title` must be between 1 and 100 symbols")
		}
	}

	if r.Description.Set {
		if r.Description.Value != nil {
			descriptionLen := len([]rune(*r.Description.Value))
			if descriptionLen < 1 || descriptionLen > 1000 {
				return fmt.Errorf("`Description` must be between 1 and 1000 symbols")
			}
		}
	}

	if r.Completed.Set {
		if r.Completed.Value == nil {
			return fmt.Errorf("`Completed` can't be NULL")
		}
	}

	return nil
}

// PatchTask 			godoc
// @Summary 			Patch Task
// @Description 		Updates information about a task that already exists in the system
// @Description 		### Three–State Logic:
// @Description 		1. **Field not passed**: `description` will be ignored, not changing DB field
// @Description 		2. **The value is explicitly passed**: `"description": "Утром в 06:30 погулять с Бобиком"` – sets a new description for the Task
// @Description 		3. **NULL was explicitly passed**: `"description": null` – clears a field in the DB (set to NULL)
// @Description 		RESTRICTIONS: fields `title` and `completed` can't be set to NULL
// @Tags 				tasks
// @Accept 				json
// @Produce 			json
// @Param 				id 			path 	 	int 				true 			"ID of the Task to be patched"
// @Param 				request 	body 	 	PatchTaskRequest 	true 			"PatchTask Request Body"
// @Success 			200 		{object} 	PatchTaskResponse 					"Task was being patched successfully"
// @Failure 			400 		{object} 	core_http_response.ErrorResponse 	"Bad Requset"
// @Failure 			404 		{object} 	core_http_response.ErrorResponse 	"Task was not found"
// @Failure 			409 		{object} 	core_http_response.ErrorResponse 	"Conflict"
// @Failure 			500 		{object} 	core_http_response.ErrorResponse 	"Internal Server Error"
// @Router 				/tasks/{id} [patch]
func (h *TasksHTTPHandler) PatchTask(rw http.ResponseWriter, r *http.Request) {
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

	var request PatchTaskRequest
	if err := core_http_request.DecodeAndValidateRequest(r, &request); err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to decode and validate HTTP request",
		)

		return
	}

	taskPatch := taskPatchFromRequest(request)

	taskDomain, err := h.tasksService.PatchTask(ctx, taskID, taskPatch)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to patch task",
		)

		return
	}

	response := PatchTaskResponse(taskDTOfromDomain(taskDomain))

	responseHandler.JSONResponse(response, http.StatusOK)
}

func taskPatchFromRequest(request PatchTaskRequest) domain.TaskPatch {
	return domain.NewTaskPatch(
		request.Title.ToDomain(),
		request.Description.ToDomain(),
		request.Completed.ToDomain(),
	)
}
