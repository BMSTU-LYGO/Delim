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
	contents := []byte("http:\n  cors_allowed_origins:\n    - http://localhost:5173\n")
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
	contents := []byte("metrics:\n  host: 0.0.0.0\n  port: 9090\n")
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
	contents := []byte("metrics:\n  host: 0.0.0.0\n  port: 70000\n")
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
	contents := []byte("metrics:\n  host: 0.0.0.0\n  port: 0\n")
	if err := os.WriteFile(path, contents, 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected validation error when metrics.host is set with metrics.port=0")
	}
}
