package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadOverridesCORSAllowedOriginsFromEnvironment(t *testing.T) {
	t.Setenv("GATEWAY_CORS_ALLOWED_ORIGINS", "https://miniapp.example,https://preview.example")

	path := filepath.Join(t.TempDir(), "gateway.yaml")
	contents := []byte("app:\n  env: local\nhttp:\n  cors_allowed_origins:\n    - http://localhost:5173\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}

	want := []string{"https://miniapp.example", "https://preview.example"}
	if !reflect.DeepEqual(cfg.HTTP.CORSAllowedOrigins, want) {
		t.Fatalf("CORS allowed origins = %q, want %q", cfg.HTTP.CORSAllowedOrigins, want)
	}
}

func TestLoadAcceptsMetricsConfig(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	contents := []byte("app:\n  env: local\nmetrics:\n  host: 0.0.0.0\n  port: 9090\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.Metrics.Port != 9090 {
		t.Fatalf("metrics.port = %d, want 9090", cfg.Metrics.Port)
	}
}

func TestLoadRejectsInvalidMetricsPort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	contents := []byte("app:\n  env: local\nmetrics:\n  host: 0.0.0.0\n  port: 70000\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := Load(path)
	if err == nil || !strings.Contains(err.Error(), "metrics.port") {
		t.Fatalf("expected metrics.port validation error, got %v", err)
	}
}

func TestLoadRejectsMetricsHostWithoutPort(t *testing.T) {
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	contents := []byte("app:\n  env: local\nmetrics:\n  host: 0.0.0.0\n  port: 0\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error when metrics.host is set with metrics.port=0")
	}
}

func strongSecretsConfig(env, session, invite, webhook, botToken string) string {
	return "app:\n  env: " + env +
		"\nauth:\n  session_secret: " + session +
		"\ninvite:\n  secret: " + invite +
		"\nmax:\n  webhook_secret: " + webhook +
		"\n  bot_token: " + botToken + "\n"
}

func writeAndLoad(t *testing.T, contents string) error {
	t.Helper()
	// Make secret-validation hermetic: configenv binds these from the
	// environment (viper), so ambient shell values would override the YAML
	// under test. Unset them for the duration of this call.
	for _, key := range []string{
		"GATEWAY_SESSION_SECRET", "GATEWAY_INVITE_SECRET",
		"MAX_WEBHOOK_SECRET", "MAX_BOT_TOKEN", "MAX_BOT_USERNAME",
		"GATEWAY_CORS_ALLOWED_ORIGINS",
	} {
		if previous, ok := os.LookupEnv(key); ok {
			k, v := key, previous
			t.Cleanup(func() { os.Setenv(k, v) })
			os.Unsetenv(key)
		}
	}
	path := filepath.Join(t.TempDir(), "gateway.yaml")
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := Load(path)
	return err
}

const (
	strongA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	strongB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	strongC = "cccccccccccccccccccccccccccccccc"
	strongD = "dddddddddddddddddddddddddddddddd"
)

func TestLoadAcceptsStrongProductionSecrets(t *testing.T) {
	if err := writeAndLoad(t, strongSecretsConfig("production", strongA, strongB, strongC, strongD)); err != nil {
		t.Fatalf("expected strong secrets to load, got %v", err)
	}
}

func TestLoadRejectsShortSecretsOutsideLocal(t *testing.T) {
	err := writeAndLoad(t, strongSecretsConfig("production", "short", strongB, strongC, strongD))
	if err == nil || !strings.Contains(err.Error(), "auth.session_secret") {
		t.Fatalf("expected session_secret length error, got %v", err)
	}
}

func TestLoadRejectsPlaceholderSecretsOutsideLocal(t *testing.T) {
	placeholder := "changeme-" + strings.Repeat("x", 24)
	err := writeAndLoad(t, strongSecretsConfig("production", placeholder, strongB, strongC, strongD))
	if err == nil || !strings.Contains(err.Error(), "placeholder") {
		t.Fatalf("expected placeholder rejection, got %v", err)
	}
}

func TestLoadRejectsReusedSecretsOutsideLocal(t *testing.T) {
	err := writeAndLoad(t, strongSecretsConfig("production", strongA, strongA, strongC, strongD))
	if err == nil || !strings.Contains(err.Error(), "distinct") {
		t.Fatalf("expected distinct-secrets error, got %v", err)
	}
}

func TestLoadRejectsMissingBotTokenOutsideLocal(t *testing.T) {
	err := writeAndLoad(t, strongSecretsConfig("production", strongA, strongB, strongC, ""))
	if err == nil || !strings.Contains(err.Error(), "max.bot_token") {
		t.Fatalf("expected bot_token error, got %v", err)
	}
}

func TestLoadAllowsEmptySecretsInLocal(t *testing.T) {
	if err := writeAndLoad(t, strongSecretsConfig("local", "", "", "", "")); err != nil {
		t.Fatalf("local environment must not enforce production secrets: %v", err)
	}
}
