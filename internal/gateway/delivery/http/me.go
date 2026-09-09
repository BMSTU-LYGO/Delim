package http

import (
	"net/http"

	corev1 "delim/pkg/gen/core/v1"
)

type currentSessionResponse struct {
	ID        int64  `json:"id"`
	MAXUserID int64  `json:"max_user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	// MAXChatID is present only when the session was established from within a
	// MAX chat using server-verified initData. It is never taken from a request
	// body, so the frontend cannot forge it.
	MAXChatID int64 `json:"max_chat_id,omitempty"`
}

func currentSession(core coreUserClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		userID, ok := userIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		response, err := core.GetUser(r.Context(), &corev1.GetUserRequest{Id: userID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		user := response.GetUser()
		if user == nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		chatID, _ := chatIDFromContext(r.Context())
		writeJSON(w, http.StatusOK, currentSessionResponse{
			ID: user.Id, MAXUserID: user.MaxUserId, FirstName: user.FirstName, LastName: user.LastName, Username: user.Username,
			MAXChatID: chatID,
		})
	}
}
