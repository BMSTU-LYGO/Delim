package http

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
)

func NewRouter() http.Handler {
	router := chi.NewRouter()
	router.Get("/health", health)
	return router
}

func health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}
