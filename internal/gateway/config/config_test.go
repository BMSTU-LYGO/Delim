package config

import (
	"os"
	"path/filepath"
	"reflect"
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
