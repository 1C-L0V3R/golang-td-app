package users_transport_http

import "github.com/1C-L0V3R/golang-td-app/internal/core/domain"

type UserDTOResponse struct {
	ID          int     `json:"id"           example:"10"`
	Version     int     `json:"version"      example:"3"`
	FullName    string  `json:"full_name"    example:"Иван Пятаков"`
	PhoneNumber *string `json:"phone_number" example:"+79998887766"`
}

func UserDTOFromDomain(user domain.User) UserDTOResponse {
	return UserDTOResponse{
		ID:          user.ID,
		Version:     user.Version,
		FullName:    user.FullName,
		PhoneNumber: user.PhoneNumber,
	}
}

func UsersDTOFromDomains(users []domain.User) []UserDTOResponse {
	usersDTO := make([]UserDTOResponse, len(users))

	for i, user := range users {
		usersDTO[i] = UserDTOFromDomain(user)
	}

	return usersDTO
}
