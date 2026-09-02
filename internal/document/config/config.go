package config

import "delim/pkg/configenv"

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Postgres PostgresConfig `mapstructure:"postgres"`
	Storage  StorageConfig  `mapstructure:"storage"`
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
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Database string `mapstructure:"database"`
	SSLMode  string `mapstructure:"sslmode"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
}

type StorageConfig struct {
	Endpoint  string `mapstructure:"endpoint"`
	Bucket    string `mapstructure:"bucket"`
	UseSSL    bool   `mapstructure:"use_ssl"`
	AccessKey string `mapstructure:"access_key"`
	SecretKey string `mapstructure:"secret_key"`
}

func Load(path string) (Config, error) {
	var cfg Config
	err := configenv.Load(path, &cfg,
		configenv.Binding{Key: "postgres.user", Env: "POSTGRES_USER"},
		configenv.Binding{Key: "postgres.password", Env: "POSTGRES_PASSWORD"},
		configenv.Binding{Key: "storage.access_key", Env: "MINIO_ROOT_USER"},
		configenv.Binding{Key: "storage.secret_key", Env: "MINIO_ROOT_PASSWORD"},
	)
	return cfg, err
}
