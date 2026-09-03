package http

import (
	"errors"
	"net/http"

	"delim/internal/gateway/auth"
	"delim/pkg/maxauth"
)

type maxLoginRequest struct {
	InitData string `json:"init_data"`
}

type maxLoginResponse struct {
	Token     string       `json:"token"`
	ExpiresIn int64        `json:"expires_in"`
	User      maxLoginUser `json:"user"`
}

type maxLoginUser struct {
	ID int64 `json:"id"`
}

func maxLogin(verifier *maxauth.InitDataVerifier, sessions *auth.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !verifier.Configured() || !sessions.Configured() {
			writeError(w, http.StatusServiceUnavailable, "max_not_configured", "MAX authentication is not configured")
			return
		}

		var request maxLoginRequest
		if err := decodeJSON(w, r, &request); err != nil || request.InitData == "" {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		initData, err := verifier.Verify(request.InitData)
		if err != nil {
			if errors.Is(err, maxauth.ErrInvalidInitData) || errors.Is(err, maxauth.ErrExpiredInitData) {
				writeError(w, http.StatusUnauthorized, "invalid_init_data", "invalid or expired MAX init data")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		token, session, err := sessions.Issue(initData.UserID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, maxLoginResponse{
			Token:     token,
			ExpiresIn: int64(session.ExpiresAt.Sub(session.IssuedAt).Seconds()),
			User:      maxLoginUser{ID: initData.UserID},
		})
	}
}
