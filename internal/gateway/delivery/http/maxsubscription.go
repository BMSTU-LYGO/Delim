package http

import (
	"context"
	"net/http"

	"delim/pkg/maxapi"
)

// maxSubscriptionStore keeps a user's private conversation with the bot.
// It deliberately has no group identifier: subscriptions belong to people.
type maxSubscriptionStore interface {
	GetPersonalSubscription(context.Context, int64) (bool, error)
	UpsertPersonalSubscription(context.Context, int64, int64) error
	DisablePersonalSubscription(context.Context, int64) error
}

type maxSubscriptionResponse struct {
	Connected bool `json:"connected"`
}

func getMaxSubscription(store maxSubscriptionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		maxUserID, ok := maxUserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		connected, err := store.GetPersonalSubscription(r.Context(), maxUserID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "subscription_unavailable", "MAX notifications are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, maxSubscriptionResponse{Connected: connected})
	}
}

// connectMaxSubscription trusts only the MAX chat context captured in the
// signed Mini App session. MAX then confirms that context is a private dialog
// before it is stored as a notification destination.
func connectMaxSubscription(store maxSubscriptionStore, maxAPI *maxapi.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		maxUserID, ok := maxUserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		chatID, ok := chatIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusBadRequest, "chat_context_required", "open the Mini App from the bot chat to enable notifications")
			return
		}
		if maxAPI == nil {
			writeError(w, http.StatusServiceUnavailable, "max_unavailable", "MAX notifications are temporarily unavailable")
			return
		}
		chat, err := maxAPI.GetChat(r.Context(), chatID)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "max_unavailable", "MAX notifications are temporarily unavailable")
			return
		}
		if chat.Type != "dialog" {
			writeError(w, http.StatusConflict, "personal_chat_required", "notifications can only be enabled from a personal bot chat")
			return
		}
		if err := store.UpsertPersonalSubscription(r.Context(), maxUserID, chatID); err != nil {
			writeError(w, http.StatusServiceUnavailable, "subscription_unavailable", "MAX notifications are temporarily unavailable")
			return
		}
		writeJSON(w, http.StatusOK, maxSubscriptionResponse{Connected: true})
	}
}

func disableMaxSubscription(store maxSubscriptionStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		maxUserID, ok := maxUserIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err := store.DisablePersonalSubscription(r.Context(), maxUserID); err != nil {
			writeError(w, http.StatusServiceUnavailable, "subscription_unavailable", "MAX notifications are temporarily unavailable")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
