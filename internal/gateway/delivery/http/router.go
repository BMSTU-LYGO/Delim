package http

import (
	"context"
	"log/slog"
	"net/http"

	"delim/internal/gateway/auth"
	"delim/internal/gateway/invite"
	"delim/pkg/maxauth"
	"github.com/go-chi/chi/v5"
)

type healthChecker interface {
	Ping(context.Context) error
}

func NewRouter(log *slog.Logger, corsAllowedOrigins []string, core coreUserClient, document, postgres healthChecker, maxAuth *maxauth.InitDataVerifier, webhookAuth *maxauth.WebhookVerifier, sessions *auth.Manager, invites *invite.Manager, inbox webhookInbox) http.Handler {
	router := chi.NewRouter()
	router.Use(requestID)
	router.Use(recoverer(log))
	router.Use(accessLog(log))
	router.Use(cors(corsAllowedOrigins))
	router.Get("/health", liveness)
	router.Get("/health/live", liveness)
	router.Get("/health/ready", readiness(core, document, postgres))
	router.Route("/api/v1", func(api chi.Router) {
		api.Post("/auth/max", maxLogin(maxAuth, sessions, invites, core))
		api.Post("/max/webhook", maxWebhook(webhookAuth, inbox))
		api.Group(func(protected chi.Router) {
			protected.Use(sessionAuth(sessions))
			protected.Get("/me", currentSession(core))
		})
	})
	return router
}
