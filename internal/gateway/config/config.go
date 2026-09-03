package config

import (
	"time"

	"delim/pkg/configenv"
)

type Config struct {
	App  AppConfig  `mapstructure:"app"`
	HTTP HTTPConfig `mapstructure:"http"`
	GRPC GRPCConfig `mapstructure:"grpc"`
	MAX  MAXConfig  `mapstructure:"max"`
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

type MAXConfig struct {
	APIURL        string        `mapstructure:"api_url"`
	InitDataTTL   time.Duration `mapstructure:"init_data_ttl"`
	BotToken      string        `mapstructure:"bot_token"`
	WebhookSecret string        `mapstructure:"webhook_secret"`
}

func Load(path string) (Config, error) {
	var cfg Config
	err := configenv.Load(path, &cfg,
		configenv.Binding{Key: "max.bot_token", Env: "MAX_BOT_TOKEN"},
		configenv.Binding{Key: "max.webhook_secret", Env: "MAX_WEBHOOK_SECRET"},
	)
	return cfg, err
}
