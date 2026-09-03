package http

import (
	"context"
	"log/slog"
	"net/http"

	"delim/internal/gateway/auth"
	"delim/pkg/maxauth"
	"github.com/go-chi/chi/v5"
)

type healthChecker interface {
	Ping(context.Context) error
}

func NewRouter(log *slog.Logger, core, document healthChecker, maxAuth *maxauth.InitDataVerifier, sessions *auth.Manager) http.Handler {
	router := chi.NewRouter()
	router.Use(requestID)
	router.Use(recoverer(log))
	router.Use(accessLog(log))
	router.Get("/health", liveness)
	router.Get("/health/live", liveness)
	router.Get("/health/ready", readiness(core, document))
	router.Post("/api/v1/auth/max", maxLogin(maxAuth, sessions))
	return router
}
