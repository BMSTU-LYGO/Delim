package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"time"

	"delim/internal/core/config"
	postgresrepo "delim/internal/core/repository/postgres"
	"delim/internal/core/service"
	"delim/internal/core/usecase"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/grpcx"
	"delim/pkg/metricsx"
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
		Host:           a.config.Postgres.Host,
		Port:           a.config.Postgres.Port,
		Database:       a.config.Postgres.Database,
		User:           a.config.Postgres.User,
		Password:       a.config.Postgres.Password,
		SSLMode:        a.config.Postgres.SSLMode,
		MaxConnections: a.config.Postgres.MaxConnections,
	})
	if err != nil {
		return err
	}
	defer pool.Close()

	recorder, shutdownMetrics, err := metricsx.New(metricsx.Config{
		Host: a.config.Metrics.Host,
		Port: a.config.Metrics.Port,
	}, metricsx.Options{Service: "core", Logger: a.logger})
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = shutdownMetrics(shutdownCtx)
	}()

	address := fmt.Sprintf("%s:%d", a.config.GRPC.Host, a.config.GRPC.Port)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", address, err)
	}

	server := grpcx.NewServer(a.logger,
		grpc.ChainUnaryInterceptor(grpcx.UnaryMetricsInterceptor(recorder)),
		grpc.ChainStreamInterceptor(grpcx.StreamMetricsInterceptor(recorder)),
	)
	store := postgresrepo.New(pool)
	users := usecase.NewUsers(store)
	groups := usecase.NewGroups(store)
	expenses := usecase.NewExpenses(store)
	ledger := usecase.NewLedger(store)
	settlements := usecase.NewSettlements(store)
	adjustments := usecase.NewAdjustments(store)
	corev1.RegisterCoreServiceServer(server, service.NewGRPCServer(users, groups, expenses, ledger, settlements, adjustments, recorder))
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
