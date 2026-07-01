package users_service

import (
	"context"
	"fmt"
)

func (s *UsersService) DeleteUser(
	ctx context.Context,
	id int,
) error {
	if err := s.usersRepository.DeleteUser(ctx, id); err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}
