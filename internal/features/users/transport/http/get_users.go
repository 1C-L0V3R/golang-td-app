package users_transport_http

import (
	"fmt"
	"net/http"

	core_logger "github.com/1C-L0V3R/golang-td-app/internal/core/logger"
	core_http_response "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/response"
	core_http_utils "github.com/1C-L0V3R/golang-td-app/internal/core/transport/http/utils"
)

type GetUsersResponse []UserDTOResponse

func (h *UsersHTTPHandler) GetUsers(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_http_response.NewHTTPResponseHadler(log, rw)

	limit, offset, err := GetLimitOffsetQueryParams(r)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get 'limit'/'offset' query param",
		)

		return
	}

	userDomains, err := h.usersService.GetUsers(ctx, limit, offset)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get users",
		)
	}

	response := GetUsersResponse(UsersDTOFromDomains(userDomains))

	responseHandler.JSONResponse(response, http.StatusOK)
}

func GetLimitOffsetQueryParams(r *http.Request) (*int, *int, error) {
	limit, err := core_http_utils.GetIntQueryParam(r, "limit")
	if err != nil {
		return nil, nil, fmt.Errorf("get 'limit' query param: %w", err)
	}

	offset, err := core_http_utils.GetIntQueryParam(r, "offset")
	if err != nil {
		return nil, nil, fmt.Errorf("get 'offset' query param: %w", err)
	}

	return limit, offset, nil
}
