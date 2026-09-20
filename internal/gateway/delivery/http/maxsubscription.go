package http

import (
	"context"
	"net/http"
	"net/url"
	"strings"
)

// maxSubscriptionStore keeps a user's private conversation with the bot.
// It deliberately has no group identifier: subscriptions belong to people.
type maxSubscriptionStore interface {
	GetPersonalSubscription(context.Context, int64) (bool, error)
	DisablePersonalSubscription(context.Context, int64) error
}

type maxSubscriptionResponse struct {
	Connected bool   `json:"connected"`
	BotURL    string `json:"bot_url,omitempty"`
}

func maxBotURL(username string) string {
	username = strings.TrimLeft(strings.TrimSpace(username), "@")
	if username == "" {
		return ""
	}
	return (&url.URL{Scheme: "https", Host: "max.ru", Path: "/" + username}).String()
}

func getMaxSubscription(store maxSubscriptionStore, botUsername string) http.HandlerFunc {
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
		writeJSON(w, http.StatusOK, maxSubscriptionResponse{Connected: connected, BotURL: maxBotURL(botUsername)})
	}
}

// Connecting opens a personal bot conversation. The bot_started webhook is
// the authoritative confirmation, so a browser cannot forge a subscription.
func connectMaxSubscription(botUsername string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := maxUserIDFromContext(r.Context()); !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		botURL := maxBotURL(botUsername)
		if botURL == "" {
			writeError(w, http.StatusServiceUnavailable, "bot_not_configured", "MAX bot is not configured")
			return
		}
		writeJSON(w, http.StatusOK, maxSubscriptionResponse{BotURL: botURL})
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
