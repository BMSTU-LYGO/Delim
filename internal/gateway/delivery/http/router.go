package http

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter(log *slog.Logger) http.Handler {
	router := chi.NewRouter()
	router.Use(requestID)
	router.Use(recoverer(log))
	router.Use(accessLog(log))
	router.Get("/health", health)
	return router
}

func health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}
