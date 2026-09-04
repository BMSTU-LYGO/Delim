package http

import (
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"delim/internal/gateway/auth"
	"delim/internal/gateway/invite"
	"delim/pkg/maxauth"
)

type maxLoginRequest struct {
	InitData string `json:"init_data"`
}

type maxLoginResponse struct {
	Token      string          `json:"token"`
	ExpiresIn  int64           `json:"expires_in"`
	User       maxLoginUser    `json:"user"`
	StartParam string          `json:"start_param,omitempty"`
	Invite     *maxLoginInvite `json:"invite,omitempty"`
}

type maxLoginUser struct {
	ID int64 `json:"id"`
}

type maxLoginInvite struct {
	GroupID   int64     `json:"group_id"`
	ExpiresAt time.Time `json:"expires_at"`
}

func maxLogin(verifier *maxauth.InitDataVerifier, sessions *auth.Manager, invites *invite.Manager) http.HandlerFunc {
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

		var inviteContext *auth.InviteContext
		var inviteResponse *maxLoginInvite
		if invite.LooksLike(initData.StartParam) {
			verified, err := invites.Verify(initData.StartParam)
			if err != nil {
				if errors.Is(err, invite.ErrNotConfigured) {
					writeError(w, http.StatusServiceUnavailable, "invite_not_configured", "invite verification is not configured")
					return
				}
				writeError(w, http.StatusUnauthorized, "invalid_start_param", "invalid or expired start parameter")
				return
			}
			inviteContext = &auth.InviteContext{GroupID: verified.GroupID, ExpiresAt: verified.ExpiresAt.Unix(), Nonce: hex.EncodeToString(verified.Nonce[:])}
			inviteResponse = &maxLoginInvite{GroupID: verified.GroupID, ExpiresAt: verified.ExpiresAt}
		}

		token, session, err := sessions.IssueWithInvite(initData.UserID, initData.UserID, inviteContext)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, maxLoginResponse{
			Token:      token,
			ExpiresIn:  int64(session.ExpiresAt.Sub(session.IssuedAt).Seconds()),
			User:       maxLoginUser{ID: initData.UserID},
			StartParam: initData.StartParam,
			Invite:     inviteResponse,
		})
	}
}
