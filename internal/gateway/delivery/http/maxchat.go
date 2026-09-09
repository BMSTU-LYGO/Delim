package http

import (
	"context"
	"errors"
	"net/http"

	postgresrepo "delim/internal/gateway/repository/postgres"
	corev1 "delim/pkg/gen/core/v1"
	"github.com/go-chi/chi/v5"
)

// chatGroupStore is the Gateway-owned MAX chat <-> Delim group binding registry.
type chatGroupStore interface {
	BindChatGroup(ctx context.Context, chatID, groupID, boundByUserID int64) (postgresrepo.ChatGroupBinding, error)
	GetChatByGroup(ctx context.Context, groupID int64) (postgresrepo.ChatGroupBinding, error)
	UnbindChatGroup(ctx context.Context, chatID int64) error
	IsChatActive(ctx context.Context, chatID int64) (bool, error)
}

// chatGroupCore verifies membership/role via Core (source of truth for perms).
type chatGroupCore interface {
	GetGroup(context.Context, *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error)
}

type maxChatResponse struct {
	ChatID        int64  `json:"chat_id"`
	GroupID       int64  `json:"group_id"`
	BoundByUserID int64  `json:"bound_by_user_id"`
	Status        string `json:"status"`
	CurrentChat   bool   `json:"current_chat"`
	ChatActive    bool   `json:"chat_active"`
}

// canManageMaxChat requires owner or admin on the group.
func canManageMaxChat(role corev1.MemberRole) bool {
	return role == corev1.MemberRole_MEMBER_ROLE_OWNER || role == corev1.MemberRole_MEMBER_ROLE_ADMIN
}

func bindGroupMaxChat(core chatGroupCore, chatGroups chatGroupStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		// chat id only ever comes from the server-verified session context.
		chatID, ok := chatIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusBadRequest, "chat_context_required", "open the Mini App from within a MAX chat to bind it")
			return
		}
		groupID, err := parseID(chi.URLParam(r, "groupID"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		group, err := core.GetGroup(r.Context(), &corev1.GetGroupRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		if !canManageMaxChat(group.GetGroup().GetCurrentUserRole()) {
			writeError(w, http.StatusForbidden, "forbidden", "owner or admin role is required")
			return
		}
		active, err := chatGroups.IsChatActive(r.Context(), chatID)
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		if !active {
			writeError(w, http.StatusConflict, "chat_not_active", "the bot is not an active member of this chat")
			return
		}
		binding, err := chatGroups.BindChatGroup(r.Context(), chatID, groupID, actorID)
		if err != nil {
			if errors.Is(err, postgresrepo.ErrGroupAlreadyBound) {
				writeError(w, http.StatusConflict, "group_already_bound", "this group is already bound to another chat")
				return
			}
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, maxChatResponse{
			ChatID: binding.ChatID, GroupID: binding.GroupID, BoundByUserID: binding.BoundByUserID,
			Status: binding.Status, CurrentChat: true, ChatActive: true,
		})
	}
}

func getGroupMaxChat(core chatGroupCore, chatGroups chatGroupStore) http.HandlerFunc {
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
		// Membership gate: a non-member cannot read the group's binding.
		group, err := core.GetGroup(r.Context(), &corev1.GetGroupRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		binding, err := chatGroups.GetChatByGroup(r.Context(), groupID)
		if errors.Is(err, postgresrepo.ErrBindingNotFound) {
			writeJSON(w, http.StatusOK, map[string]any{"bound": false, "role_can_manage": canManageMaxChat(group.GetGroup().GetCurrentUserRole())})
			return
		}
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		sessionChatID, _ := chatIDFromContext(r.Context())
		active, _ := chatGroups.IsChatActive(r.Context(), binding.ChatID)
		writeJSON(w, http.StatusOK, maxChatResponse{
			ChatID: binding.ChatID, GroupID: binding.GroupID, BoundByUserID: binding.BoundByUserID,
			Status: binding.Status, CurrentChat: sessionChatID == binding.ChatID && sessionChatID != 0, ChatActive: active,
		})
	}
}

func unbindGroupMaxChat(core chatGroupCore, chatGroups chatGroupStore) http.HandlerFunc {
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
		group, err := core.GetGroup(r.Context(), &corev1.GetGroupRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		if !canManageMaxChat(group.GetGroup().GetCurrentUserRole()) {
			writeError(w, http.StatusForbidden, "forbidden", "owner or admin role is required")
			return
		}
		binding, err := chatGroups.GetChatByGroup(r.Context(), groupID)
		if err != nil {
			if errors.Is(err, postgresrepo.ErrBindingNotFound) {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			writeDownstreamError(w, err)
			return
		}
		if err := chatGroups.UnbindChatGroup(r.Context(), binding.ChatID); err != nil {
			writeDownstreamError(w, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
