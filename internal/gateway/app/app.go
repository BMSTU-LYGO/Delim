package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	coreclient "delim/internal/gateway/client/core"
	documentclient "delim/internal/gateway/client/document"
	"delim/internal/gateway/config"
	httpdelivery "delim/internal/gateway/delivery/http"
	"delim/pkg/maxapi"
	"delim/pkg/maxauth"
)

type App struct {
	config  config.Config
	logger  *slog.Logger
	maxAPI  *maxapi.Client
	maxAuth *maxauth.Verifier
}

func New(cfg config.Config, log *slog.Logger) *App {
	return &App{
		config:  cfg,
		logger:  log,
		maxAPI:  maxapi.New(cfg.MAX.APIURL, cfg.MAX.BotToken),
		maxAuth: maxauth.New(cfg.MAX.WebhookSecret),
	}
}

func (a *App) Run(ctx context.Context) error {
	core, err := coreclient.New(a.config.GRPC.CoreAddress)
	if err != nil {
		return err
	}
	defer core.Close()

	document, err := documentclient.New(a.config.GRPC.DocumentAddress)
	if err != nil {
		return err
	}
	defer document.Close()

	pingCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if err := core.Ping(pingCtx); err != nil {
		return err
	}
	if err := document.Ping(pingCtx); err != nil {
		return err
	}
	a.logger.Info("grpc connectivity verified", "core", a.config.GRPC.CoreAddress, "document", a.config.GRPC.DocumentAddress)

	address := fmt.Sprintf("%s:%d", a.config.HTTP.Host, a.config.HTTP.Port)
	server := &http.Server{
		Addr:              address,
		Handler:           httpdelivery.NewRouter(),
		ReadHeaderTimeout: 5 * time.Second,
	}
	a.logger.Info("service started", "address", address)

	errCh := make(chan error, 1)
	go func() { errCh <- server.ListenAndServe() }()

	select {
	case <-ctx.Done():
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		return server.Shutdown(shutdownCtx)
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("serve http: %w", err)
	}
}
