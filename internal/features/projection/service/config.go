package projection_service

import (
	"fmt"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	ConsumerGroup string `envconfig:"CONSUMER_GROUP" default:"todoapp-projector"`
}

func NewConfig() (Config, error) {
	var config Config

	if err := envconfig.Process("PROJECTION", &config); err != nil {
		return Config{}, fmt.Errorf("process envconfig: %w", err)
	}

	return config, nil
}

func NewConfigMust() Config {
	config, err := NewConfig()
	if err != nil {
		err = fmt.Errorf("get Projection config: %w", err)
		panic(err)
	}

	return config
}
