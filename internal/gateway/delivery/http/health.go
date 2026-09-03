package http

import (
	"context"
	"net/http"
	"time"
)

const readinessTimeout = 2 * time.Second

type readinessResponse struct {
	Status   string `json:"status"`
	Core     string `json:"core"`
	Document string `json:"document"`
}

type dependencyResult struct {
	name string
	err  error
}

func liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readiness(core, document healthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		results := make(chan dependencyResult, 2)
		go func() { results <- dependencyResult{name: "core", err: core.Ping(ctx)} }()
		go func() { results <- dependencyResult{name: "document", err: document.Ping(ctx)} }()

		response := readinessResponse{Status: "unavailable", Core: "unavailable", Document: "unavailable"}
		for range 2 {
			select {
			case result := <-results:
				if result.err == nil {
					if result.name == "core" {
						response.Core = "ok"
					} else {
						response.Document = "ok"
					}
				}
			case <-ctx.Done():
				writeJSON(w, http.StatusServiceUnavailable, response)
				return
			}
		}

		status := http.StatusServiceUnavailable
		if response.Core == "ok" && response.Document == "ok" {
			response.Status = "ok"
			status = http.StatusOK
		}
		writeJSON(w, status, response)
	}
}
