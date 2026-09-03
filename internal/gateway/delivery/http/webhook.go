package http

import (
	"context"
	"encoding/json"
	"io"
	"net/http"

	"delim/internal/gateway/maxupdate"
	"delim/pkg/maxauth"
)

const maxWebhookBody = 1 << 20

type updateDispatcher interface {
	Dispatch(context.Context, maxupdate.Update)
}

func maxWebhook(verifier *maxauth.WebhookVerifier, dispatcher updateDispatcher) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !verifier.Configured() {
			writeError(w, http.StatusServiceUnavailable, "max_not_configured", "MAX webhook is not configured")
			return
		}
		if !verifier.Verify(r.Header.Get("X-Max-Bot-Api-Secret")) {
			writeError(w, http.StatusUnauthorized, "invalid_webhook_secret", "invalid webhook secret")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBody)
		decoder := json.NewDecoder(r.Body)
		var update maxupdate.Update
		if err := decoder.Decode(&update); err != nil || update.UpdateType == "" {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		if err := decoder.Decode(&struct{}{}); err != io.EOF {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}

		dispatcher.Dispatch(r.Context(), update)
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
