package users_postgres_repository

import core_postgres_pool "github.com/1C-L0V3R/golang-td-app/internal/core/repository/postgres/pool"

type UsersRepository struct {
	pool core_postgres_pool.Pool
}

func NewUsersRepository(
	pool core_postgres_pool.Pool,
) *UsersRepository {
	return &UsersRepository{
		pool: pool,
	}
}
