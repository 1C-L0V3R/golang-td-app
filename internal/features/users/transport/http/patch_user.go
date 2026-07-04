package users_transport_http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_http_request "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/request"
	core_http_response "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/response"
	core_http_types "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/types"
)

type PatchUserRequest struct {
	FullName    core_http_types.Nullable[string] `json:"full_name"    swaggertype:"string" example:"Mamut Rahal"`
	PhoneNumber core_http_types.Nullable[string] `json:"phone_number" swaggertype:"string" example:"+71112223344"`
}

func (r *PatchUserRequest) Validate() error {
	if r.FullName.Set {
		if r.FullName.Value == nil {
			return fmt.Errorf("`FullName` can't be NULL")
		}

		fullNameLen := len([]rune(*r.FullName.Value))
		if fullNameLen < 3 || fullNameLen > 100 {
			return fmt.Errorf("`FullName` must be between 3 and 100 symbols")
		}
	}

	if r.PhoneNumber.Set {
		if r.PhoneNumber.Value != nil {
			phoneNumberLen := len([]rune(*r.PhoneNumber.Value))
			if phoneNumberLen < 10 || phoneNumberLen > 15 {
				return fmt.Errorf("`PhoneNumber` must be between 10 and 15 symbols")
			}

			if !strings.HasPrefix(*r.PhoneNumber.Value, "+") {
				return fmt.Errorf("`PhoneNumber` must startswith '+' symbol")
			}
		}
	}

	return nil
}

type PatchUserResponse UserDTOResponse

// PatchUser            godoc
// @Summary  			Patching User
// @Description     	Patching existing User by their ID
// @Description     	### Three-state logic:
// @Description     	1. **Field not passed**: `phone_number` ignored, value in DB not changing
// @Description     	2. **The value is explicitly passed**: `"phone_number": "+71112223344"` – sets a new phone number in the DB
// @Description     	3. **NULL Passed**: `"phone_number": null` – clears a field in the DB (set to NULL)
// @Description			RESTRICTIONS: `full_name` field can't be NULL
// @Tags 				users
// @Accept 				json
// @Produce 			json
// @Param 				id path int true 									"ID of User to patch"
// @Param 				request body PatchUserRequest true 					"Patch User Request Body"
// @Success 			200 {object} PatchUserResponse 						"User successfully patched"
// @Failure 			400 {object} core_http_response.ErrorResponse 		"Bad Request"
// @Failure 			404 {object} core_http_response.ErrorResponse 		"User Not Found"
// @Failure 			409 {object} core_http_response.ErrorResponse 		"Conflict"
// @Failure 			500 {object} core_http_response.ErrorResponse 		"Internal Server Error"
// @Router 				/users/{id} [patch]
func (h *UsersHTTPHandler) PatchUser(rw http.ResponseWriter, r *http.Request) {
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

	var request PatchUserRequest
	if err := core_http_request.DecodeAndValidateRequest(r, &request); err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to decode and validate request",
		)

		return
	}

	userPatch := userPatchFromRequest(request)

	userDomain, err := h.usersService.PatchUser(ctx, userID, userPatch)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to patch user",
		)

		return
	}

	response := PatchUserResponse(UserDTOFromDomain(userDomain))

	responseHandler.JSONResponse(response, http.StatusOK)
}

func userPatchFromRequest(request PatchUserRequest) domain.UserPatch {
	return domain.NewUserPatch(
		request.FullName.ToDomain(),
		request.PhoneNumber.ToDomain(),
	)
}
