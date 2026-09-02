package config

import "delim/pkg/configenv"

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Postgres PostgresConfig `mapstructure:"postgres"`
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

func Load(path string) (Config, error) {
	var cfg Config
	err := configenv.Load(path, &cfg,
		configenv.Binding{Key: "postgres.user", Env: "POSTGRES_USER"},
		configenv.Binding{Key: "postgres.password", Env: "POSTGRES_PASSWORD"},
	)
	return cfg, err
}
