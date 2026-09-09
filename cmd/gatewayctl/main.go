package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"strings"

	"delim/internal/gateway/auth"
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
	if len(args) == 0 {
		return usageError()
	}
	cfg, err := config.Load("configs/gateway.yaml")
	if err != nil {
		return fmt.Errorf("load gateway config: %w", err)
	}
	switch args[0] {
	case "max":
		return runMAX(ctx, cfg, args[1:])
	case "dev":
		return runDev(cfg, args[1:])
	default:
		return usageError()
	}
}

func runMAX(ctx context.Context, cfg config.Config, args []string) error {
	if len(args) != 1 || (args[0] != "check" && args[0] != "setup") {
		return usageError()
	}
	if cfg.MAX.BotToken == "" {
		if args[0] == "check" {
			return maxDiagnosticsOffline(cfg)
		}
		return errors.New("MAX_BOT_TOKEN is required")
	}
	client := maxapi.New(cfg.MAX.APIURL, cfg.MAX.BotToken)
	bot, err := client.GetMe(ctx)
	if err != nil {
		return fmt.Errorf("check MAX bot token: %w", err)
	}
	if args[0] == "check" {
		return maxDiagnosticsOnline(ctx, client, cfg, bot)
	}
	if err := setup(ctx, client, cfg); err != nil {
		return err
	}
	fmt.Printf("%s (%d): MAX setup complete\n", botName(bot), bot.UserID)
	return nil
}

// maxStatus is a bounded, secret-free line in the integration checklist.
type maxStatus struct {
	label  string
	state  string
	detail string
}

func (s maxStatus) String() string {
	if s.detail != "" {
		return fmt.Sprintf("  %-16s %-8s %s", s.label+":", s.state, s.detail)
	}
	return fmt.Sprintf("  %-16s %s", s.label+":", s.state)
}

func configChecks(cfg config.Config) []maxStatus {
	return []maxStatus{
		secretPresence("bot token", cfg.MAX.BotToken),
		secretPresence("webhook secret", cfg.MAX.WebhookSecret),
		{label: "webhook url", state: urlState(cfg.MAX.WebhookURL), detail: cfg.MAX.WebhookURL},
		secretPresence("mini app url", cfg.MAX.MiniAppURL),
	}
}

func secretPresence(label, value string) maxStatus {
	if strings.TrimSpace(value) == "" {
		return maxStatus{label: label, state: "MISSING"}
	}
	return maxStatus{label: label, state: "set"}
}

func urlState(value string) string {
	if strings.TrimSpace(value) == "" {
		return "MISSING"
	}
	if validateWebhookURL(value) != nil {
		return "INVALID"
	}
	return "ok"
}

// maxDiagnosticsOffline prints the config-only checklist when no bot token is
// configured, so CI/dev without credentials still get an actionable report.
func maxDiagnosticsOffline(cfg config.Config) error {
	fmt.Println("MAX integration diagnostics (offline: MAX_BOT_TOKEN not set)")
	for _, line := range configChecks(cfg) {
		fmt.Println(line)
	}
	return nil
}

// maxDiagnosticsOnline verifies the live bot identity plus webhook subscription
// shape against local config, without ever printing the token or secret value.
func maxDiagnosticsOnline(ctx context.Context, client *maxapi.Client, cfg config.Config, bot maxapi.Bot) error {
	fmt.Println("MAX integration diagnostics")
	botLine := maxStatus{label: "getMe", state: "ok", detail: fmt.Sprintf("%s (%d)", botName(bot), bot.UserID)}
	fmt.Println(botLine)

	usernameState := "unset"
	usernameDetail := ""
	if cfg.MAX.BotUsername != "" {
		usernameDetail = "@" + cfg.MAX.BotUsername
		if bot.Username != nil && *bot.Username == cfg.MAX.BotUsername {
			usernameState = "match"
		} else {
			usernameState = "MISMATCH"
		}
	}
	fmt.Println(maxStatus{label: "bot username", state: usernameState, detail: usernameDetail})

	for _, line := range configChecks(cfg) {
		fmt.Println(line)
	}

	subscriptions, err := client.GetSubscriptions(ctx)
	if err != nil {
		fmt.Println(maxStatus{label: "webhook sub", state: "ERROR", detail: err.Error()})
		return nil
	}
	if len(subscriptions) == 0 {
		fmt.Println(maxStatus{label: "webhook sub", state: "MISSING", detail: "no subscriptions"})
		return nil
	}
	matched := false
	for _, subscription := range subscriptions {
		detail := subscription.URL
		if cfg.MAX.WebhookURL != "" && subscription.URL == cfg.MAX.WebhookURL {
			matched = true
		}
		fmt.Println(maxStatus{label: "subscription", state: "present", detail: detail})
		fmt.Println(maxStatus{label: "update types", state: updateTypesState(subscription.UpdateTypes), detail: strings.Join(subscription.UpdateTypes, ",")})
	}
	if cfg.MAX.WebhookURL != "" && !matched {
		fmt.Println(maxStatus{label: "expected url", state: "MISMATCH", detail: "configured webhook URL has no subscription"})
	}
	return nil
}

func updateTypesState(present []string) string {
	if len(present) == 0 {
		return "all"
	}
	have := make(map[string]struct{}, len(present))
	for _, value := range present {
		have[value] = struct{}{}
	}
	for _, expected := range webhookUpdateTypes {
		if _, ok := have[expected]; !ok {
			return "INCOMPLETE"
		}
	}
	return "ok"
}

func runDev(cfg config.Config, args []string) error {
	if len(args) == 0 || args[0] != "session" {
		return usageError()
	}
	if cfg.App.Env != "local" {
		return errors.New("dev session is available only when app.env=local")
	}
	flags := flag.NewFlagSet("gatewayctl dev session", flag.ContinueOnError)
	userID := flags.Int64("user-id", 0, "Core user ID")
	maxUserID := flags.Int64("max-user-id", 0, "MAX user ID")
	if err := flags.Parse(args[1:]); err != nil {
		return err
	}
	if flags.NArg() != 0 || *userID <= 0 || *maxUserID <= 0 {
		return errors.New("usage: gatewayctl dev session --user-id <core_id> --max-user-id <id>")
	}
	sessions := auth.NewManager(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL)
	token, _, err := sessions.Issue(*userID, *maxUserID)
	if err != nil {
		return fmt.Errorf("issue local session: %w", err)
	}
	fmt.Println(token)
	return nil
}

func usageError() error {
	return errors.New("usage: gatewayctl max <check|setup> | gatewayctl dev session --user-id <core_id> --max-user-id <id>")
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
