package http

import (
	"context"
	"encoding/hex"
	"errors"
	"net/http"
	"time"

	"delim/internal/gateway/auth"
	"delim/internal/gateway/invite"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/maxauth"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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
	ID        int64 `json:"id"`
	MAXUserID int64 `json:"max_user_id"`
}

type maxLoginInvite struct {
	Status    string     `json:"status"`
	GroupID   int64      `json:"group_id,omitempty"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
}

type coreUserClient interface {
	healthChecker
	UpsertUser(context.Context, *corev1.UpsertUserRequest) (*corev1.UpsertUserResponse, error)
	GetUser(context.Context, *corev1.GetUserRequest) (*corev1.GetUserResponse, error)
	JoinGroup(context.Context, *corev1.JoinGroupRequest) (*corev1.JoinGroupResponse, error)
}

func maxLogin(verifier *maxauth.InitDataVerifier, sessions *auth.Manager, invites *invite.Manager, core coreUserClient) http.HandlerFunc {
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
		upserted, err := core.UpsertUser(r.Context(), &corev1.UpsertUserRequest{
			MaxUserId: initData.UserID,
			FirstName: initData.FirstName,
			LastName:  initData.LastName,
			Username:  initData.Username,
		})
		if err != nil {
			if status.Code(err) == codes.Unavailable {
				writeError(w, http.StatusServiceUnavailable, "core_unavailable", "service unavailable")
				return
			}
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		if upserted.GetUser().GetId() == 0 {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}

		var inviteContext *auth.InviteContext
		var inviteResponse *maxLoginInvite
		if invite.LooksLike(initData.StartParam) {
			verified, err := invites.Verify(initData.StartParam)
			if err != nil {
				inviteStatus := "invalid"
				if errors.Is(err, invite.ErrExpiredInvite) {
					inviteStatus = "expired"
				} else if errors.Is(err, invite.ErrNotConfigured) {
					inviteStatus = "unavailable"
				}
				inviteResponse = &maxLoginInvite{Status: inviteStatus}
			} else {
				expiresAt := verified.ExpiresAt
				inviteContext = &auth.InviteContext{GroupID: verified.GroupID, ExpiresAt: expiresAt.Unix(), Nonce: hex.EncodeToString(verified.Nonce[:])}
				inviteResponse = &maxLoginInvite{Status: "join_failed", GroupID: verified.GroupID, ExpiresAt: &expiresAt}
				if _, err := core.JoinGroup(r.Context(), &corev1.JoinGroupRequest{ActorUserId: upserted.User.Id, GroupId: verified.GroupID}); err == nil {
					inviteResponse.Status = "joined"
				}
			}
		}

		token, session, err := sessions.IssueWithInvite(upserted.User.Id, initData.UserID, inviteContext)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			return
		}
		writeJSON(w, http.StatusOK, maxLoginResponse{
			Token:      token,
			ExpiresIn:  int64(session.ExpiresAt.Sub(session.IssuedAt).Seconds()),
			User:       maxLoginUser{ID: upserted.User.Id, MAXUserID: initData.UserID},
			StartParam: initData.StartParam,
			Invite:     inviteResponse,
		})
	}
}
