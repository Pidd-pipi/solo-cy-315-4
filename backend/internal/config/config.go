package config

import (
	"fmt"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config holds all runtime configuration injected through environment variables.
type Config struct {
	Env        string        `env:"ENV" envDefault:"development"`
	ServerPort int           `env:"SERVER_PORT" envDefault:"8080"`
	DBPath     string        `env:"DB_PATH" envDefault:"./data/gbschedule.db"`
	LogLevel   string        `env:"LOG_LEVEL" envDefault:"info"`
	DBTimeout  time.Duration `env:"DB_TIMEOUT" envDefault:"10s"`
}

// Load parses environment variables into a Config instance.
func Load() (*Config, error) {
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cfg, nil
}
