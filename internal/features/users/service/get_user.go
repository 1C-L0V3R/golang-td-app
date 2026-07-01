package users_service

import (
	"context"
	"fmt"

	"github.com/1C-L0V3R/golang-td-app/internal/core/domain"
)

func (s *UsersService) GetUser(
	ctx context.Context,
	id int,
) (domain.User, error) {
	user, err := s.usersRepository.GetUser(ctx, id)
	if err != nil {
		return domain.User{}, fmt.Errorf(
			"get user from repository: %w",
			err,
		)
	}

	return user, nil
}
