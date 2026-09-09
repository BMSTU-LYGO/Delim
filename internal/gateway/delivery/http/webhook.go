package http

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"net/http"

	"delim/internal/gateway/maxupdate"
	"delim/pkg/maxauth"
	"delim/pkg/metricsx"
)

const maxWebhookBody = 1 << 20

type webhookInbox interface {
	IngestUpdate(context.Context, string, string, *int64, []byte) error
}

func maxWebhook(verifier *maxauth.WebhookVerifier, inbox webhookInbox, recorder *metricsx.Recorder) http.HandlerFunc {
	observe := func(result string) {
		if recorder != nil {
			recorder.ObserveWebhook(result)
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !verifier.Configured() {
			observe("failed")
			writeError(w, http.StatusServiceUnavailable, "max_not_configured", "MAX webhook is not configured")
			return
		}
		if !verifier.Verify(r.Header.Get("X-Max-Bot-Api-Secret")) {
			observe("failed")
			writeError(w, http.StatusUnauthorized, "invalid_webhook_secret", "invalid webhook secret")
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, maxWebhookBody)
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			observe("failed")
			var maxErr *http.MaxBytesError
			if errors.As(err, &maxErr) {
				writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "webhook body is too large")
				return
			}
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		update, err := maxupdate.Parse(raw)
		if err != nil {
			observe("failed")
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}

		hash := sha256.Sum256(raw)
		var chatID *int64
		if update.ChatID != 0 {
			chatID = &update.ChatID
		}
		if err := inbox.IngestUpdate(r.Context(), fmt.Sprintf("%x", hash), string(update.UpdateType), chatID, raw); err != nil {
			// On duplicate event_key the underlying store returns no error
			// because it is silently ignored; any non-nil error here is a
			// real storage failure rather than a duplicate webhook.
			observe("failed")
			writeError(w, http.StatusServiceUnavailable, "storage_unavailable", "update could not be stored")
			return
		}
		observe("accepted")
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	}
}
