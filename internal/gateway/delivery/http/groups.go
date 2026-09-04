package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	corev1 "delim/pkg/gen/core/v1"
	"github.com/go-chi/chi/v5"
)

type groupClient interface {
	coreUserClient
	CreateGroup(context.Context, *corev1.CreateGroupRequest) (*corev1.CreateGroupResponse, error)
	GetGroup(context.Context, *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error)
	ListGroups(context.Context, *corev1.ListGroupsRequest) (*corev1.ListGroupsResponse, error)
}

type createGroupRequest struct {
	Name string `json:"name"`
}

type groupResponse struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	OwnerID         int64     `json:"owner_id"`
	Status          string    `json:"status"`
	CurrentUserRole string    `json:"current_user_role"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type groupListResponse struct {
	Groups     []groupResponse `json:"groups"`
	NextCursor int64           `json:"next_cursor,omitempty"`
}

func createGroup(core groupClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		var request createGroupRequest
		if err := decodeJSON(w, r, &request); err != nil || request.Name == "" {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		response, err := core.CreateGroup(r.Context(), &corev1.CreateGroupRequest{ActorUserId: actorID, Name: request.Name})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, groupToResponse(response.GetGroup()))
	}
}

func getGroup(core groupClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
		if err != nil || groupID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		response, err := core.GetGroup(r.Context(), &corev1.GetGroupRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, groupToResponse(response.GetGroup()))
	}
}

func listGroups(core groupClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		limit := int64(50)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			var err error
			limit, err = strconv.ParseInt(raw, 10, 32)
			if err != nil || limit <= 0 || limit > 100 {
				writeError(w, http.StatusBadRequest, "invalid_argument", "invalid limit")
				return
			}
		}
		var cursor int64
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			var err error
			cursor, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || cursor < 0 {
				writeError(w, http.StatusBadRequest, "invalid_argument", "invalid cursor")
				return
			}
		}
		response, err := core.ListGroups(r.Context(), &corev1.ListGroupsRequest{ActorUserId: actorID, Page: &corev1.PageRequest{Limit: int32(limit), CursorId: cursor}})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		groups := make([]groupResponse, 0, len(response.GetGroups()))
		for _, group := range response.GetGroups() {
			groups = append(groups, groupToResponse(group))
		}
		writeJSON(w, http.StatusOK, groupListResponse{Groups: groups, NextCursor: response.GetPage().GetNextCursorId()})
	}
}

func groupToResponse(group *corev1.Group) groupResponse {
	if group == nil {
		return groupResponse{}
	}
	return groupResponse{
		ID: group.Id, Name: group.Name, OwnerID: group.OwnerId,
		Status: groupStatusName(group.Status), CurrentUserRole: memberRoleName(group.CurrentUserRole),
		CreatedAt: group.CreatedAt.AsTime(), UpdatedAt: group.UpdatedAt.AsTime(),
	}
}

func groupStatusName(status corev1.GroupStatus) string {
	switch status {
	case corev1.GroupStatus_GROUP_STATUS_ACTIVE:
		return "active"
	case corev1.GroupStatus_GROUP_STATUS_ARCHIVED:
		return "archived"
	default:
		return "unspecified"
	}
}

func memberRoleName(role corev1.MemberRole) string {
	switch role {
	case corev1.MemberRole_MEMBER_ROLE_OWNER:
		return "owner"
	case corev1.MemberRole_MEMBER_ROLE_ADMIN:
		return "admin"
	case corev1.MemberRole_MEMBER_ROLE_MEMBER:
		return "member"
	default:
		return "unspecified"
	}
}
