package core_kafka

import (
	"fmt"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Brokers      []string      `envconfig:"BROKERS"       required:"true"`
	TopicPrefix  string        `envconfig:"TOPIC_PREFIX"  default:"todoapp"`
	WriteTimeout time.Duration `envconfig:"WRITE_TIMEOUT" default:"10s"`
}

func NewConfig() (Config, error) {
	var config Config

	if err := envconfig.Process("KAFKA", &config); err != nil {
		return Config{}, fmt.Errorf("process envconfig: %w", err)
	}

	return config, nil
}

func NewConfigMust() Config {
	config, err := NewConfig()
	if err != nil {
		err = fmt.Errorf("get Kafka config: %w", err)
		panic(err)
	}

	return config
}

const (
	topicUsers = "users"
	topicTasks = "tasks"
)

func (c Config) UsersTopic() string {
	return fmt.Sprintf("%s.%s", c.TopicPrefix, topicUsers)
}

func (c Config) TasksTopic() string {
	return fmt.Sprintf("%s.%s", c.TopicPrefix, topicTasks)
}
