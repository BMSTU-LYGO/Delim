// Command fixture provisions local-only actors for the Mini App E2E suite.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"delim/internal/gateway/auth"
	coreclient "delim/internal/gateway/client/core"
	gatewayconfig "delim/internal/gateway/config"
	corev1 "delim/pkg/gen/core/v1"
)

const (
	ownerMAXID  = int64(9_091_000_001)
	memberMAXID = int64(9_091_000_002)
)

type actor struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Token string `json:"token"`
}

type fixture struct {
	Member actor `json:"member"`
	Owner  actor `json:"owner"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if err := loadLocalEnv(".env"); err != nil {
		return err
	}
	cfg, err := gatewayconfig.Load("configs/gateway.yaml")
	if err != nil {
		return fmt.Errorf("load Gateway config: %w", err)
	}
	if cfg.App.Env != "local" {
		return errors.New("E2E fixtures are available only when app.env=local")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := coreclient.New(envOr("E2E_CORE_ADDR", "localhost:50051"))
	if err != nil {
		return fmt.Errorf("connect to Core: %w", err)
	}
	defer client.Close()
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("Core dev stack is not ready: %w", err)
	}

	sessions := auth.NewManager(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL)
	owner, err := provisionActor(ctx, client, sessions, ownerMAXID, "Алиса E2E")
	if err != nil {
		return err
	}
	member, err := provisionActor(ctx, client, sessions, memberMAXID, "Борис E2E")
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(fixture{Member: member, Owner: owner})
}

func provisionActor(
	ctx context.Context,
	client *coreclient.Client,
	sessions *auth.Manager,
	maxUserID int64,
	name string,
) (actor, error) {
	response, err := client.UpsertUser(ctx, &corev1.UpsertUserRequest{
		FirstName: name,
		MaxUserId: maxUserID,
		Username:  strings.ToLower(strings.ReplaceAll(name, " ", "_")),
	})
	if err != nil {
		return actor{}, fmt.Errorf("provision %s: %w", name, err)
	}
	userID := response.GetUser().GetId()
	token, _, err := sessions.Issue(userID, maxUserID)
	if err != nil {
		return actor{}, fmt.Errorf("issue %s session: %w", name, err)
	}
	return actor{ID: userID, Name: name, Token: token}, nil
}

func loadLocalEnv(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return fmt.Errorf("invalid environment entry in %s", path)
		}
		key = strings.TrimSpace(key)
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		value = strings.Trim(strings.TrimSpace(value), "\"'")
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s: %w", key, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}
	return nil
}

func envOr(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}
