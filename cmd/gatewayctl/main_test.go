package main

import (
	"strings"
	"testing"

	"delim/pkg/maxapi"

	"delim/internal/gateway/config"
)

func TestUpdateTypesState(t *testing.T) {
	cases := []struct {
		name    string
		present []string
		want    string
	}{
		{"empty means all", nil, "all"},
		{"complete", webhookUpdateTypes, "ok"},
		{"missing one", []string{"bot_added", "message_created"}, "INCOMPLETE"},
		{"extra values ok", append(append([]string{}, webhookUpdateTypes...), "future_type"), "ok"},
	}
	for _, entry := range cases {
		t.Run(entry.name, func(t *testing.T) {
			if got := updateTypesState(entry.present); got != entry.want {
				t.Fatalf("updateTypesState(%v) = %q, want %q", entry.present, got, entry.want)
			}
		})
	}
}

func TestURLState(t *testing.T) {
	cases := map[string]string{
		"":                 "MISSING",
		"http://x/api":     "INVALID",
		"https://x/api":    "ok",
		"https://x:8443/a": "INVALID",
		"not a url":        "INVALID",
	}
	for value, want := range cases {
		if got := urlState(value); got != want {
			t.Fatalf("urlState(%q) = %q, want %q", value, got, want)
		}
	}
}

func TestConfigChecksNeverRevealSecretValues(t *testing.T) {
	cfg := config.Config{}
	cfg.MAX.BotToken = "super-secret-token-value"
	cfg.MAX.WebhookSecret = "super-secret-webhook-value"
	cfg.MAX.MiniAppURL = "https://mini.example/app"
	cfg.MAX.WebhookURL = "https://hooks.example/webhook"
	for _, line := range configChecks(cfg) {
		rendered := line.String()
		for _, secret := range []string{"super-secret-token-value", "super-secret-webhook-value"} {
			if strings.Contains(rendered, secret) {
				t.Fatalf("diagnostics leaked secret in %q", rendered)
			}
		}
	}
}

func TestCommandSetState(t *testing.T) {
	if got := commandSetState(expectedBotCommands); got != "ok" {
		t.Fatalf("full command set = %q, want ok", got)
	}
	if got := commandSetState([]maxapi.BotCommand{{Name: "start"}}); got != "INCOMPLETE" {
		t.Fatalf("partial command set = %q, want INCOMPLETE", got)
	}
}
