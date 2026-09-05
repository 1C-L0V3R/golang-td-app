package outbox_service

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	PollInterval time.Duration `envconfig:"POLL_INTERVAL" default:"1s"`
	BatchSize    int           `envconfig:"BATCH_SIZE"    default:"100"`
	Retention    time.Duration `envconfig:"RETENTION"     default:"168h"`
}

func NewConfig() (Config, error) {
	var config Config

	if err := envconfig.Process("OUTBOX", &config); err != nil {
		return Config{}, fmt.Errorf("process envconfig: %w", err)
	}

	return config, nil
}

func NewConfigMust() Config {
	config, err := NewConfig()
	if err != nil {
		err = fmt.Errorf("get Outbox relay config: %w", err)
		panic(err)
	}

	return config
}
