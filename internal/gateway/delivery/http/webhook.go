package http

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"

	"delim/internal/gateway/maxupdate"
	"delim/pkg/maxauth"
)

const maxWebhookBody = 1 << 20

type webhookInbox interface {
	IngestUpdate(context.Context, string, string, *int64, []byte) error
}

func maxWebhook(verifier *maxauth.WebhookVerifier, inbox webhookInbox) http.HandlerFunc {
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
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		update, err := maxupdate.Parse(raw)
		if err != nil {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}

		hash := sha256.Sum256(raw)
		var chatID *int64
		if update.ChatID != 0 {
			chatID = &update.ChatID
		}
		if err := inbox.IngestUpdate(r.Context(), fmt.Sprintf("%x", hash), string(update.UpdateType), chatID, raw); err != nil {
			writeError(w, http.StatusServiceUnavailable, "storage_unavailable", "update could not be stored")
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
