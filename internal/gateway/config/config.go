package config

import (
	"fmt"
	"strings"
	"time"

	"delim/pkg/configenv"
)

type Config struct {
	App       AppConfig       `mapstructure:"app"`
	HTTP      HTTPConfig      `mapstructure:"http"`
	GRPC      GRPCConfig      `mapstructure:"grpc"`
	Document  DocumentConfig  `mapstructure:"document"`
	Auth      AuthConfig      `mapstructure:"auth"`
	Invite    InviteConfig    `mapstructure:"invite"`
	MAX       MAXConfig       `mapstructure:"max"`
	Postgres  PostgresConfig  `mapstructure:"postgres"`
	Metrics   MetricsConfig   `mapstructure:"metrics"`
	RateLimit RateLimitConfig `mapstructure:"rate_limit"`
}

type DocumentConfig struct {
	UploadMaxSizeMB int64 `mapstructure:"upload_max_size_mb"`
}

func (c DocumentConfig) UploadMaxSizeBytes() int64 {
	return c.UploadMaxSizeMB * 1024 * 1024
}

type AppConfig struct {
	Name string `mapstructure:"name"`
	Env  string `mapstructure:"env"`
}

type HTTPConfig struct {
	Host               string        `mapstructure:"host"`
	Port               int           `mapstructure:"port"`
	ReadHeaderTimeout  time.Duration `mapstructure:"read_header_timeout"`
	ReadTimeout        time.Duration `mapstructure:"read_timeout"`
	WriteTimeout       time.Duration `mapstructure:"write_timeout"`
	IdleTimeout        time.Duration `mapstructure:"idle_timeout"`
	MaxHeaderBytes     int           `mapstructure:"max_header_bytes"`
	CORSAllowedOrigins []string      `mapstructure:"cors_allowed_origins"`
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
	Secret string        `mapstructure:"secret"`
	TTL    time.Duration `mapstructure:"ttl"`
}

type MAXConfig struct {
	APIURL        string        `mapstructure:"api_url"`
	InitDataTTL   time.Duration `mapstructure:"init_data_ttl"`
	BotUsername   string        `mapstructure:"bot_username"`
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

type MetricsConfig struct {
	Host string `mapstructure:"host"`
	Port int    `mapstructure:"port"`
}

type RateLimitConfig struct {
	AuthPerMinute    int `mapstructure:"auth_per_minute"`
	WebhookPerMinute int `mapstructure:"webhook_per_minute"`
	UploadPerMinute  int `mapstructure:"upload_per_minute"`
	APIPerMinute     int `mapstructure:"api_per_minute"`
	MaxKeys          int `mapstructure:"max_keys"`
}

func Load(path string) (Config, error) {
	var cfg Config
	err := configenv.Load(path, &cfg,
		configenv.Binding{Key: "auth.session_secret", Env: "GATEWAY_SESSION_SECRET"},
		configenv.Binding{Key: "http.cors_allowed_origins", Env: "GATEWAY_CORS_ALLOWED_ORIGINS"},
		configenv.Binding{Key: "invite.secret", Env: "GATEWAY_INVITE_SECRET"},
		configenv.Binding{Key: "max.bot_username", Env: "MAX_BOT_USERNAME"},
		configenv.Binding{Key: "max.bot_token", Env: "MAX_BOT_TOKEN"},
		configenv.Binding{Key: "max.webhook_secret", Env: "MAX_WEBHOOK_SECRET"},
		configenv.Binding{Key: "max.webhook_url", Env: "MAX_WEBHOOK_URL"},
		configenv.Binding{Key: "postgres.user", Env: "POSTGRES_USER"},
		configenv.Binding{Key: "postgres.password", Env: "POSTGRES_PASSWORD"},
	)
	if err != nil {
		return cfg, err
	}
	if err := cfg.validate(); err != nil {
		return cfg, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	if err := validateMetrics(c.Metrics); err != nil {
		return err
	}
	if err := validateRateLimit(c.RateLimit); err != nil {
		return err
	}
	if c.App.Env == "local" {
		return nil
	}
	return validateProductionSecrets(c)
}

// minProductionSecretLength is the minimum accepted length for every HMAC
// signing secret outside the local environment.
const minProductionSecretLength = 32

// weakSecretFragments reject obvious placeholder values regardless of case.
var weakSecretFragments = []string{"changeme", "change_me", "replace-me", "replace_me", "placeholder", "example", "your-", "your_"}

func validateProductionSecrets(c Config) error {
	signingSecrets := map[string]string{
		"auth.session_secret": c.Auth.SessionSecret,
		"invite.secret":       c.Invite.Secret,
		"max.webhook_secret":  c.MAX.WebhookSecret,
	}
	for name, value := range signingSecrets {
		if len(value) < minProductionSecretLength {
			return fmt.Errorf("%s must be at least %d characters when app.env is not local", name, minProductionSecretLength)
		}
		for _, fragment := range weakSecretFragments {
			if strings.Contains(strings.ToLower(value), fragment) {
				return fmt.Errorf("%s must not use a placeholder value when app.env is not local", name)
			}
		}
	}
	// Session, invite, and webhook signing keys must never share material.
	if c.Auth.SessionSecret == c.Invite.Secret ||
		c.Auth.SessionSecret == c.MAX.WebhookSecret ||
		c.Invite.Secret == c.MAX.WebhookSecret {
		return fmt.Errorf("auth.session_secret, invite.secret, and max.webhook_secret must be distinct when app.env is not local")
	}
	if c.MAX.BotToken == "" {
		return fmt.Errorf("max.bot_token is required when app.env is not local")
	}
	if c.MAX.BotToken == c.Auth.SessionSecret || c.MAX.BotToken == c.Invite.Secret || c.MAX.BotToken == c.MAX.WebhookSecret {
		return fmt.Errorf("max.bot_token must not be reused as a signing secret when app.env is not local")
	}
	return nil
}

func validateRateLimit(r RateLimitConfig) error {
	for name, value := range map[string]int{
		"rate_limit.auth_per_minute":    r.AuthPerMinute,
		"rate_limit.webhook_per_minute": r.WebhookPerMinute,
		"rate_limit.upload_per_minute":  r.UploadPerMinute,
		"rate_limit.api_per_minute":     r.APIPerMinute,
		"rate_limit.max_keys":           r.MaxKeys,
	} {
		if value < 0 {
			return fmt.Errorf("%s must not be negative", name)
		}
	}
	return nil
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
