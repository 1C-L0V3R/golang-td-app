package core_mongo

import (
	"fmt"
	"net/url"
	"time"

	"github.com/kelseyhightower/envconfig"
)

type Config struct {
	Host     string        `envconfig:"HOST"     required:"true"`
	Port     string        `envconfig:"PORT"     default:"27017"`
	User     string        `envconfig:"USER"     required:"true"`
	Password string        `envconfig:"PASSWORD" required:"true"`
	Database string        `envconfig:"DB"       required:"true"`
	Timeout  time.Duration `envconfig:"TIMEOUT"  default:"10s"`
}

func NewConfig() (Config, error) {
	var config Config

	if err := envconfig.Process("MONGO", &config); err != nil {
		return Config{}, fmt.Errorf("process envconfig: %w", err)
	}

	return config, nil
}

func NewConfigMust() Config {
	config, err := NewConfig()
	if err != nil {
		err = fmt.Errorf("get Mongo client config: %w", err)
		panic(err)
	}

	return config
}

func (c Config) URI() string {
	return fmt.Sprintf(
		"mongodb://%s:%s@%s:%s/?authSource=admin",
		url.QueryEscape(c.User),
		url.QueryEscape(c.Password),
		c.Host,
		c.Port,
	)
}
