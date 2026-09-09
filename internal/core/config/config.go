package config

import (
	"fmt"

	"delim/pkg/configenv"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	Metrics  MetricsConfig  `mapstructure:"metrics"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type GRPCConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type PostgresConfig struct {
	Host           string `mapstructure:"host"`
	Port           int    `mapstructure:"port"`
	Database       string `mapstructure:"database"`
	SSLMode        string `mapstructure:"sslmode"`
	MaxConnections int32  `mapstructure:"max_connections"`
	User           string `mapstructure:"user"`
	Password       string `mapstructure:"password"`
}

type MetricsConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

func Load(path string) (Config, error) {
	var cfg Config
	err := configenv.Load(path, &cfg,
		configenv.Binding{Key: "postgres.user", Env: "POSTGRES_USER"},
		configenv.Binding{Key: "postgres.password", Env: "POSTGRES_PASSWORD"},
	)
	if err != nil {
		return cfg, err
	}
	if err := validateMetrics(cfg.Metrics); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func validateMetrics(m MetricsConfig) error {
	if m.Port < 0 || m.Port > 65535 {
		return fmt.Errorf("metrics.port must be between 0 and 65535")
	}
	if m.Port == 0 && m.Host != "" {
		return fmt.Errorf("metrics.host is not used when metrics.port is 0")
	}
	return nil
}
