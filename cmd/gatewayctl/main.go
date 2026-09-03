package main

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"strings"

	"delim/internal/gateway/config"
	"delim/pkg/maxapi"
)

var webhookUpdateTypes = []string{
	"bot_added",
	"bot_removed",
	"bot_started",
	"user_added",
	"user_removed",
	"message_created",
	"message_callback",
}

func main() {
	if err := run(context.Background(), os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) != 2 || args[0] != "max" || (args[1] != "check" && args[1] != "setup") {
		return errors.New("usage: gatewayctl max <check|setup>")
	}
	cfg, err := config.Load("configs/gateway.yaml")
	if err != nil {
		return fmt.Errorf("load gateway config: %w", err)
	}
	if cfg.MAX.BotToken == "" {
		return errors.New("MAX_BOT_TOKEN is required")
	}
	client := maxapi.New(cfg.MAX.APIURL, cfg.MAX.BotToken)
	bot, err := client.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("check MAX bot token: %w", err)
	}
	if args[1] == "check" {
		fmt.Printf("%s (%d)\n", botName(bot), bot.UserID)
		return nil
	}
	if err := setup(ctx, client, cfg); err != nil {
		return err
	}
	fmt.Printf("%s (%d): MAX setup complete\n", botName(bot), bot.UserID)
	return nil
}

func setup(ctx context.Context, client *maxapi.Client, cfg config.Config) error {
	if err := validateWebhookURL(cfg.MAX.WebhookURL); err != nil {
		return err
	}
	if cfg.MAX.WebhookSecret == "" {
		return errors.New("MAX_WEBHOOK_SECRET is required")
	}
	commands := []maxapi.BotCommand{
		{Name: "start", Description: "Начать работу с Делим"},
		{Name: "help", Description: "Помощь по Делим"},
	}
	if err := client.SetBotCommands(ctx, commands); err != nil {
		return fmt.Errorf("set MAX bot commands: %w", err)
	}
	subscriptions, err := client.GetSubscriptions(ctx)
	if err != nil {
		return fmt.Errorf("get MAX webhook subscriptions: %w", err)
	}
	for _, subscription := range subscriptions {
		if subscription.URL != cfg.MAX.WebhookURL {
			if err := client.DeleteSubscription(ctx, subscription.URL); err != nil {
				return fmt.Errorf("delete old MAX webhook subscription: %w", err)
			}
		}
	}
	if err := client.CreateSubscription(ctx, maxapi.CreateSubscriptionRequest{
		URL:         cfg.MAX.WebhookURL,
		Secret:      cfg.MAX.WebhookSecret,
		UpdateTypes: webhookUpdateTypes,
	}); err != nil {
		return fmt.Errorf("create MAX webhook subscription: %w", err)
	}
	return nil
}

func validateWebhookURL(value string) error {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || (parsed.Port() != "" && parsed.Port() != "443") {
		return errors.New("MAX_WEBHOOK_URL must be a standard HTTPS URL")
	}
	return nil
}

func botName(bot maxapi.Bot) string {
	if strings.TrimSpace(bot.FirstName) != "" {
		return bot.FirstName
	}
	if bot.Username != nil && *bot.Username != "" {
		return *bot.Username
	}
	return "MAX bot"
}
