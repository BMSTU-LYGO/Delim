package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"delim/internal/gateway/auth"
	coreclient "delim/internal/gateway/client/core"
	documentclient "delim/internal/gateway/client/document"
	"delim/internal/gateway/config"
	httpdelivery "delim/internal/gateway/delivery/http"
	"delim/internal/gateway/invite"
	"delim/internal/gateway/launch"
	"delim/internal/gateway/maxupdate"
	"delim/internal/gateway/membersync"
	"delim/internal/gateway/notifications"
	"delim/internal/gateway/ratelimit"
	postgresrepo "delim/internal/gateway/repository/postgres"
	"delim/pkg/maxapi"
	"delim/pkg/maxauth"
	"delim/pkg/metricsx"
	"delim/pkg/postgresx"
)

type App struct {
	config      config.Config
	logger      *slog.Logger
	maxAPI      *maxapi.Client
	maxAuth     *maxauth.InitDataVerifier
	webhookAuth *maxauth.WebhookVerifier
	sessions    *auth.Manager
	invites     *invite.Manager
	launches    *launch.Manager
}

func New(cfg config.Config, log *slog.Logger) *App {
	return &App{
		config:      cfg,
		logger:      log,
		maxAPI:      maxapi.NewInstrumentedClient(cfg.MAX.APIURL, cfg.MAX.BotToken, log),
		maxAuth:     maxauth.NewInitDataVerifier(cfg.MAX.BotToken, cfg.MAX.InitDataTTL),
		webhookAuth: maxauth.NewWebhookVerifier(cfg.MAX.WebhookSecret),
		sessions:    auth.NewManager(cfg.Auth.SessionSecret, cfg.Auth.SessionTTL),
		invites:     invite.NewManager(cfg.Invite.Secret),
		launches:    launch.NewManager(cfg.Launch.Secret, cfg.Launch.TTL),
	}
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
	}, metricsx.Options{Service: "gateway", Logger: a.logger})
	if err != nil {
		return err
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = shutdownMetrics(shutdownCtx)
	}()
	a.maxAPI.SetMetrics(recorder)

	store := postgresrepo.New(pool)

	core, err := coreclient.New(a.config.GRPC.CoreAddress, recorder)
	if err != nil {
		return err
	}
	defer core.Close()

	document, err := documentclient.New(a.config.GRPC.DocumentAddress, recorder)
	if err != nil {
		return err
	}
	defer document.Close()

	updates := maxupdate.NewDispatcher(store, a.maxAPI, core, a.launches, a.logger, recorder, a.config.MAX.MiniAppURL)
	worker := maxupdate.NewWorker(store, updates, a.logger, recorder)
	workerCtx, stopWorker := context.WithCancel(ctx)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		worker.Run(workerCtx)
	}()
	defer func() {
		stopWorker()
		<-workerDone
	}()

	notificationWorker := notifications.NewWorker(store, a.maxAPI, a.launches, a.config.MAX.MiniAppURL, a.logger, recorder)
	notificationCtx, stopNotifications := context.WithCancel(ctx)
	notificationDone := make(chan struct{})
	go func() {
		defer close(notificationDone)
		notificationWorker.Run(notificationCtx)
	}()
	defer func() {
		stopNotifications()
		<-notificationDone
	}()

	sync := membersync.New(store, a.maxAPI, core)
	notifier := notifications.NewNotifier(store)

	a.logger.Info("grpc clients created", "core", a.config.GRPC.CoreAddress, "document", a.config.GRPC.DocumentAddress)

	rl := a.config.RateLimit
	limits := httpdelivery.RateLimits{
		Auth:    ratelimit.New(rl.AuthPerMinute, rl.MaxKeys),
		Webhook: ratelimit.New(rl.WebhookPerMinute, rl.MaxKeys),
		Upload:  ratelimit.New(rl.UploadPerMinute, rl.MaxKeys),
		API:     ratelimit.New(rl.APIPerMinute, rl.MaxKeys),
	}

	address := fmt.Sprintf("%s:%d", a.config.HTTP.Host, a.config.HTTP.Port)
	server := &http.Server{
		Addr:              address,
		Handler:           httpdelivery.NewRouter(a.logger, a.config.HTTP.CORSAllowedOrigins, a.config.Document.UploadMaxSizeBytes(), a.config.Invite.TTL, a.config.MAX.BotUsername, core, document, store, a.maxAuth, a.webhookAuth, a.sessions, a.invites, a.launches, store, store, sync, notifier, recorder, limits),
		ReadHeaderTimeout: a.config.HTTP.ReadHeaderTimeout,
		ReadTimeout:       a.config.HTTP.ReadTimeout,
		WriteTimeout:      a.config.HTTP.WriteTimeout,
		IdleTimeout:       a.config.HTTP.IdleTimeout,
		MaxHeaderBytes:    a.config.HTTP.MaxHeaderBytes,
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
