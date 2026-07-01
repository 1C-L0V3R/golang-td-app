package core_postgres_pool

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Host     string        `envconfig:"HOST"      required:"true"`
	Port     string        `envconfig:"PORT"      default:"5432"`
	User     string        `envconfig:"USER"      required:"true"`
	Password string        `envconfig:"PASSWORD"  required:"true"`
	Database string        `envconfig:"DB"        required:"true"`
	Timeout  time.Duration `envconfig:"TIMEOUT"   required:"true"`
}

func NewConfig() (Config, error) {
	var config Config

	if err := envconfig.Process("POSTGRES", &config); err != nil {
		return Config{}, fmt.Errorf("process envconfig: %w", err)
	}

	return config, nil
}

func NewConfigMust() Config {
	config, err := NewConfig()
	if err != nil {
		err = fmt.Errorf("get Postgres connection pool config: %w", err)
		panic(err)
	}

	return config
}
