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
	JoinGroup(context.Context, *corev1.JoinGroupRequest) (*corev1.JoinGroupResponse, error)
	ListGroupMembers(context.Context, *corev1.ListGroupMembersRequest) (*corev1.ListGroupMembersResponse, error)
	AddGroupMembers(context.Context, *corev1.AddGroupMembersRequest) (*corev1.AddGroupMembersResponse, error)
	UpdateMemberRole(context.Context, *corev1.UpdateMemberRoleRequest) (*corev1.UpdateMemberRoleResponse, error)
	ArchiveGroup(context.Context, *corev1.ArchiveGroupRequest) (*corev1.ArchiveGroupResponse, error)
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

type addGroupMembersRequest struct {
	UserIDs []int64 `json:"user_ids"`
}

type updateMemberRoleRequest struct {
	Role string `json:"role"`
}

type groupMemberResponse struct {
	GroupID  int64               `json:"group_id"`
	UserID   int64               `json:"user_id"`
	Role     string              `json:"role"`
	JoinedAt time.Time           `json:"joined_at"`
	User     *groupMemberUserDTO `json:"user,omitempty"`
}

type groupMemberUserDTO struct {
	ID        int64  `json:"id"`
	MAXUserID int64  `json:"max_user_id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
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

func joinGroup(core groupClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil || groupID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		response, err := core.JoinGroup(r.Context(), &corev1.JoinGroupRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, groupMemberToResponse(response.GetMember()))
	}
}

func listGroupMembers(core groupClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil || groupID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		response, err := core.ListGroupMembers(r.Context(), &corev1.ListGroupMembersRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		members := make([]groupMemberResponse, 0, len(response.GetMembers()))
		for _, member := range response.GetMembers() {
			members = append(members, groupMemberToResponse(member))
		}
		writeJSON(w, http.StatusOK, members)
	}
}

func addGroupMembers(core groupClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil || groupID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		var request addGroupMembersRequest
		if err := decodeJSON(w, r, &request); err != nil || len(request.UserIDs) == 0 {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		for _, userID := range request.UserIDs {
			if userID <= 0 {
				writeError(w, http.StatusBadRequest, "invalid_argument", "invalid user id")
				return
			}
		}
		response, err := core.AddGroupMembers(r.Context(), &corev1.AddGroupMembersRequest{ActorUserId: actorID, GroupId: groupID, UserIds: request.UserIDs})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		members := make([]groupMemberResponse, 0, len(response.GetMembers()))
		for _, member := range response.GetMembers() {
			members = append(members, groupMemberToResponse(member))
		}
		writeJSON(w, http.StatusOK, members)
	}
}

func updateMemberRole(core groupClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		groupID, groupErr := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
		userID, userErr := strconv.ParseInt(chi.URLParam(r, "userID"), 10, 64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if groupErr != nil || groupID <= 0 || userErr != nil || userID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group or user id")
			return
		}
		var request updateMemberRoleRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		role, valid := memberRoleFromName(request.Role)
		if !valid {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid member role")
			return
		}
		response, err := core.UpdateMemberRole(r.Context(), &corev1.UpdateMemberRoleRequest{ActorUserId: actorID, GroupId: groupID, UserId: userID, Role: role})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, groupMemberToResponse(response.GetMember()))
	}
}

func archiveGroup(core groupClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil || groupID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		response, err := core.ArchiveGroup(r.Context(), &corev1.ArchiveGroupRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, groupToResponse(response.GetGroup()))
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

func memberRoleFromName(role string) (corev1.MemberRole, bool) {
	switch role {
	case "owner":
		return corev1.MemberRole_MEMBER_ROLE_OWNER, true
	case "admin":
		return corev1.MemberRole_MEMBER_ROLE_ADMIN, true
	case "member":
		return corev1.MemberRole_MEMBER_ROLE_MEMBER, true
	default:
		return corev1.MemberRole_MEMBER_ROLE_UNSPECIFIED, false
	}
}

func groupMemberToResponse(member *corev1.GroupMember) groupMemberResponse {
	if member == nil {
		return groupMemberResponse{}
	}
	response := groupMemberResponse{GroupID: member.GroupId, UserID: member.UserId, Role: memberRoleName(member.Role), JoinedAt: member.JoinedAt.AsTime()}
	if user := member.GetUser(); user != nil {
		response.User = &groupMemberUserDTO{ID: user.Id, MAXUserID: user.MaxUserId, FirstName: user.FirstName, LastName: user.LastName, Username: user.Username}
	}
	return response
}
