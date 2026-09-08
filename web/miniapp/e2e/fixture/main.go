// Command fixture provisions local-only actors for the Mini App E2E suite.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"delim/internal/devtools"
	"delim/internal/gateway/auth"
	coreclient "delim/internal/gateway/client/core"
	gatewayconfig "delim/internal/gateway/config"
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
	if err := devtools.LoadLocalEnv(".env"); err != nil {
		return err
	}
	cfg, err := gatewayconfig.Load("configs/gateway.yaml")
	if err != nil {
		return fmt.Errorf("load Gateway config: %w", err)
	}
	if err := devtools.RequireLocal(cfg.App.Env); err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	client, err := coreclient.New(devtools.EnvOr("E2E_CORE_ADDR", "localhost:50051"))
	if err != nil {
		return fmt.Errorf("connect to Core: %w", err)
	}
	defer client.Close()
	if err := client.Ping(ctx); err != nil {
		return fmt.Errorf("Core dev stack is not ready: %w", err)
	}

	sessions := auth.NewManager(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL)
	owner, err := devtools.ProvisionActor(
		ctx, client, sessions, ownerMAXID, "Алиса E2E", "alisa_e2e",
	)
	if err != nil {
		return err
	}
	member, err := devtools.ProvisionActor(
		ctx, client, sessions, memberMAXID, "Борис E2E", "boris_e2e",
	)
	if err != nil {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(fixture{
		Member: actor{ID: member.UserID, Name: member.Name, Token: member.Token},
		Owner:  actor{ID: owner.UserID, Name: owner.Name, Token: owner.Token},
	})
}
