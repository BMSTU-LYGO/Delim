package config

import (
	"time"

	"delim/pkg/configenv"
)

type Config struct {
	App      AppConfig      `mapstructure:"app"`
	HTTP     HTTPConfig     `mapstructure:"http"`
	GRPC     GRPCConfig     `mapstructure:"grpc"`
	Auth     AuthConfig     `mapstructure:"auth"`
	Invite   InviteConfig   `mapstructure:"invite"`
	MAX      MAXConfig      `mapstructure:"max"`
	Postgres PostgresConfig `mapstructure:"postgres"`
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type HTTPConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type GRPCConfig struct {
	CoreAddress     string `mapstructure:"core_addr"`
	DocumentAddress string `mapstructure:"document_addr"`
}

type AuthConfig struct {
	SessionTTL    time.Duration `mapstructure:"session_ttl"`
	SessionSecret string        `mapstructure:"session_secret"`
}

type InviteConfig struct {
	Secret string `mapstructure:"secret"`
}

type MAXConfig struct {
	APIURL        string        `mapstructure:"api_url"`
	InitDataTTL   time.Duration `mapstructure:"init_data_ttl"`
	BotToken      string        `mapstructure:"bot_token"`
	WebhookSecret string        `mapstructure:"webhook_secret"`
	WebhookURL    string        `mapstructure:"webhook_url"`
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
		configenv.Binding{Key: "auth.session_secret", Env: "GATEWAY_SESSION_SECRET"},
		configenv.Binding{Key: "invite.secret", Env: "GATEWAY_INVITE_SECRET"},
		configenv.Binding{Key: "max.bot_token", Env: "MAX_BOT_TOKEN"},
		configenv.Binding{Key: "max.webhook_secret", Env: "MAX_WEBHOOK_SECRET"},
		configenv.Binding{Key: "max.webhook_url", Env: "MAX_WEBHOOK_URL"},
		configenv.Binding{Key: "postgres.user", Env: "POSTGRES_USER"},
		configenv.Binding{Key: "postgres.password", Env: "POSTGRES_PASSWORD"},
	)
	return cfg, err
}
