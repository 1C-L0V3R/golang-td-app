package core_postgres_pool

import "errors"

var (
	ErrNoRows             = errors.New("no rows")
	ErrViolatesForeingKey = errors.New("violates foreing key")
	ErrUnknown            = errors.New("unknown")
)
