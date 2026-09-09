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
	OCR      string `json:"ocr"`
	Postgres string `json:"postgres"`
}

// documentReadiness checks reachability (Ping) plus the separately reported OCR
// subsystem state. A degraded/unavailable OCR never flips the Document
// component or overall status; it is surfaced only in the "ocr" field.
type documentReadiness interface {
	healthChecker
	OCRStatus(ctx context.Context) (string, error)
}

type dependencyResult struct {
	name string
	err  error
}

func liveness(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func readiness(core healthChecker, document documentReadiness, postgres healthChecker) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readinessTimeout)
		defer cancel()

		results := make(chan dependencyResult, 3)
		go func() { results <- dependencyResult{name: "core", err: core.Ping(ctx)} }()
		go func() { results <- dependencyResult{name: "document", err: document.Ping(ctx)} }()
		go func() { results <- dependencyResult{name: "postgres", err: postgres.Ping(ctx)} }()

		response := readinessResponse{Status: "unavailable", Core: "unavailable", Document: "unavailable", OCR: "unknown", Postgres: "unavailable"}
		for range 3 {
			select {
			case result := <-results:
				if result.err == nil {
					if result.name == "core" {
						response.Core = "ok"
					} else if result.name == "document" {
						response.Document = "ok"
					} else {
						response.Postgres = "ok"
					}
				}
			case <-ctx.Done():
				writeJSON(w, http.StatusServiceUnavailable, response)
				return
			}
		}

		if response.Document == "ok" {
			if ocr, err := document.OCRStatus(ctx); err == nil && ocr != "" {
				response.OCR = ocr
			}
		}

		status := http.StatusServiceUnavailable
		if response.Core == "ok" && response.Document == "ok" && response.Postgres == "ok" {
			response.Status = "ok"
			status = http.StatusOK
		}
		writeJSON(w, status, response)
	}
}
