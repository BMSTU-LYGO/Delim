package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"delim/internal/document/app"
	"delim/internal/document/config"
	"delim/pkg/logger"
)

func main() {
	configPath := "configs/document.yaml"
	if len(os.Args) > 1 {
		configPath = os.Args[1]
	}

	cfg, err := config.Load(configPath)
	if err != nil {
		slog.Error("load config", "error", err)
		os.Exit(1)
	}

	log := logger.New(cfg.App.Name, cfg.App.Env)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	if err := app.New(cfg, log).Run(ctx); err != nil {
		log.Error("service stopped", "error", err)
		os.Exit(1)
	}
}
