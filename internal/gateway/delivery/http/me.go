package http

import "net/http"

type currentSessionResponse struct {
	MAXUserID int64 `json:"max_user_id"`
}

func currentSession(w http.ResponseWriter, r *http.Request) {
	userID, ok := maxUserIDFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
		return
	}
	writeJSON(w, http.StatusOK, currentSessionResponse{MAXUserID: userID})
}
