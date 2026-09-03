package http

import (
	"context"
	"log/slog"
	"net/http"

	"delim/internal/gateway/auth"
	"delim/internal/gateway/maxupdate"
	"delim/pkg/maxauth"
	"github.com/go-chi/chi/v5"
)

type healthChecker interface {
	Ping(context.Context) error
}

func NewRouter(log *slog.Logger, core, document, postgres healthChecker, maxAuth *maxauth.InitDataVerifier, webhookAuth *maxauth.WebhookVerifier, sessions *auth.Manager, updates *maxupdate.Dispatcher) http.Handler {
	router := chi.NewRouter()
	router.Use(requestID)
	router.Use(recoverer(log))
	router.Use(accessLog(log))
	router.Get("/health", liveness)
	router.Get("/health/live", liveness)
	router.Get("/health/ready", readiness(core, document, postgres))
	router.Route("/api/v1", func(api chi.Router) {
		api.Post("/auth/max", maxLogin(maxAuth, sessions))
		api.Post("/max/webhook", maxWebhook(webhookAuth, updates))
		api.Group(func(protected chi.Router) {
			protected.Use(sessionAuth(sessions))
			protected.Get("/me", currentSession)
		})
	})
	return router
}
