package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"

	"delim/internal/document/config"
	"delim/internal/document/service"
	"delim/internal/document/storage"
	documentv1 "delim/pkg/gen/document/v1"
	"delim/pkg/grpcx"
	"delim/pkg/postgresx"
	"google.golang.org/grpc"
)

type App struct {
	config config.Config
	logger *slog.Logger
}

func New(cfg config.Config, log *slog.Logger) *App {
	return &App{config: cfg, logger: log}
}

func (a *App) Run(ctx context.Context) error {
	pool, err := postgresx.Open(ctx, postgresx.Config{
		Host:     a.config.Postgres.Host,
		Port:     a.config.Postgres.Port,
		Database: a.config.Postgres.Database,
		User:     a.config.Postgres.User,
		Password: a.config.Postgres.Password,
		SSLMode:  a.config.Postgres.SSLMode,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	if _, err := storage.New(
		a.config.Storage.Endpoint,
		a.config.Storage.AccessKey,
		a.config.Storage.SecretKey,
		a.config.Storage.Bucket,
		a.config.Storage.UseSSL,
	); err != nil {
		return err
	}

	address := fmt.Sprintf("%s:%d", a.config.GRPC.Host, a.config.GRPC.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}

	server := grpcx.NewServer()
	documentv1.RegisterDocumentServiceServer(server, service.NewGRPCServer())
	a.logger.Info("service started", "address", address)

	errCh := make(chan error, 1)
	go func() { errCh <- server.Serve(listener) }()

	select {
	case <-ctx.Done():
		server.GracefulStop()
		return nil
	case err := <-errCh:
		if errors.Is(err, grpc.ErrServerStopped) {
			return nil
		}
		return fmt.Errorf("serve grpc: %w", err)
	}
}
