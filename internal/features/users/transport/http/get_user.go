package users_transport_http

import (
	"net/http"

	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_http_request "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/request"
	core_http_response "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/response"
)

type GetUserResponse UserDTOResponse

// GetUser           godoc
// @Summary 		 Getting User
// @Description      Getting User by their ID
// @Tags 			 users
// @Produce 		 json
// @Param            id path int true 				                 "ID of the recipient user"
// @Success          200 {object} GetUserResponse   				 "User founded successfully"
// @Failure          400 {object} core_http_response.ErrorResponse   "Bad Request"
// @Failure          404 {object} core_http_response.ErrorResponse   "User not found"
// @Failure          500 {object} core_http_response.ErrorResponse   "Internal Server Error"
// @Router           /users/{id} [get]
func (h *UsersHTTPHandler) GetUser(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_http_response.NewHTTPResponseHadler(log, rw)

	userID, err := core_http_request.GetIntPathValue(r, "id")
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get userID path value",
		)

		return
	}

	user, err := h.usersService.GetUser(ctx, userID)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get user",
		)

		return
	}

	response := GetUserResponse(UserDTOFromDomain(user))

	responseHandler.JSONResponse(response, http.StatusOK)
}
