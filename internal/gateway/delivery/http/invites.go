package http

import (
	"net/http"
	"net/url"
	"strings"
	"time"

	"delim/internal/gateway/invite"
	corev1 "delim/pkg/gen/core/v1"
	"github.com/go-chi/chi/v5"
)

type inviteResponse struct {
	StartParam string    `json:"start_param"`
	DeepLink   string    `json:"deep_link,omitempty"`
	ExpiresAt  time.Time `json:"expires_at"`
}

func createGroupInvite(core receiptCoreClient, invites *invite.Manager, ttl time.Duration, botUsername string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		groupID, err := parseID(chi.URLParam(r, "groupID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		response, err := core.GetGroup(r.Context(), &corev1.GetGroupRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		group := response.GetGroup()
		if group == nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if group.GetCurrentUserRole() != corev1.MemberRole_MEMBER_ROLE_OWNER && group.GetCurrentUserRole() != corev1.MemberRole_MEMBER_ROLE_ADMIN {
			writeError(w, http.StatusForbidden, "permission_denied", "permission denied")
			return
		}
		if ttl <= 0 {
			writeError(w, http.StatusServiceUnavailable, "invite_not_configured", "invitation is not configured")
			return
		}
		expiresAt := time.Now().Add(ttl)
		startParam, value, err := invites.Issue(groupID, expiresAt)
		if err != nil {
			writeError(w, http.StatusServiceUnavailable, "invite_not_configured", "invitation is not configured")
			return
		}
		writeJSON(w, http.StatusCreated, inviteResponse{
			StartParam: startParam,
			DeepLink:   maxDeepLink(botUsername, startParam),
			ExpiresAt:  value.ExpiresAt,
		})
	}
}

func maxDeepLink(botUsername, startParam string) string {
	username := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(botUsername), "@"))
	if username == "" {
		return ""
	}
	query := url.Values{}
	query.Set("startapp", startParam)
	return (&url.URL{Scheme: "https", Host: "max.ru", Path: "/" + username, RawQuery: query.Encode()}).String()
}
