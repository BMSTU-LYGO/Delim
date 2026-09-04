package service

import (
	"context"
	"delim/internal/core/domain"
	corev1 "delim/pkg/gen/core/v1"
)

type GroupService interface {
	Create(context.Context, int64, string) (domain.Group, error)
	Get(context.Context, int64, int64) (domain.Group, error)
	List(context.Context, int64, int64, int32) ([]domain.Group, error)
	Join(context.Context, int64, int64) (domain.GroupMember, error)
	UpdateRole(context.Context, int64, int64, int64, domain.MemberRole) (domain.GroupMember, error)
	Archive(context.Context, int64, int64) (domain.Group, error)
}

func (s *GRPCServer) ArchiveGroup(ctx context.Context, req *corev1.ArchiveGroupRequest) (*corev1.ArchiveGroupResponse, error) {
	group, err := s.groups.Archive(ctx, req.GetActorUserId(), req.GetGroupId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.ArchiveGroupResponse{Group: groupToProto(group)}, nil
}

func (s *GRPCServer) JoinGroup(ctx context.Context, req *corev1.JoinGroupRequest) (*corev1.JoinGroupResponse, error) {
	member, err := s.groups.Join(ctx, req.GetActorUserId(), req.GetGroupId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.JoinGroupResponse{Member: memberToProto(member)}, nil
}

func (s *GRPCServer) UpdateMemberRole(ctx context.Context, req *corev1.UpdateMemberRoleRequest) (*corev1.UpdateMemberRoleResponse, error) {
	member, err := s.groups.UpdateRole(ctx, req.GetActorUserId(), req.GetGroupId(), req.GetUserId(), memberRoleFromProto(req.GetRole()))
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.UpdateMemberRoleResponse{Member: memberToProto(member)}, nil
}

func memberToProto(member domain.GroupMember) *corev1.GroupMember {
	return &corev1.GroupMember{GroupId: member.GroupID, UserId: member.UserID, Role: memberRoleToProto(member.Role), JoinedAt: timeToProto(member.JoinedAt)}
}
func memberRoleFromProto(role corev1.MemberRole) domain.MemberRole {
	switch role {
	case corev1.MemberRole_MEMBER_ROLE_ADMIN:
		return domain.RoleAdmin
	case corev1.MemberRole_MEMBER_ROLE_MEMBER:
		return domain.RoleMember
	case corev1.MemberRole_MEMBER_ROLE_OWNER:
		return domain.RoleOwner
	default:
		return ""
	}
}

func (s *GRPCServer) GetGroup(ctx context.Context, req *corev1.GetGroupRequest) (*corev1.GetGroupResponse, error) {
	group, err := s.groups.Get(ctx, req.GetActorUserId(), req.GetGroupId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.GetGroupResponse{Group: groupToProto(group)}, nil
}

func (s *GRPCServer) ListGroups(ctx context.Context, req *corev1.ListGroupsRequest) (*corev1.ListGroupsResponse, error) {
	var cursor int64
	var limit int32
	if req.GetPage() != nil {
		cursor, limit = req.GetPage().GetCursorId(), req.GetPage().GetLimit()
	}
	groups, err := s.groups.List(ctx, req.GetActorUserId(), cursor, limit)
	if err != nil {
		return nil, toGRPCError(err)
	}
	response := &corev1.ListGroupsResponse{Groups: make([]*corev1.Group, 0, len(groups)), Page: &corev1.PageResponse{}}
	for _, group := range groups {
		response.Groups = append(response.Groups, groupToProto(group))
	}
	if len(groups) > 0 {
		response.Page.NextCursorId = groups[len(groups)-1].ID
	}
	return response, nil
}

func (s *GRPCServer) CreateGroup(ctx context.Context, req *corev1.CreateGroupRequest) (*corev1.CreateGroupResponse, error) {
	group, err := s.groups.Create(ctx, req.GetActorUserId(), req.GetName())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.CreateGroupResponse{Group: groupToProto(group)}, nil
}

func groupToProto(group domain.Group) *corev1.Group {
	return &corev1.Group{Id: group.ID, Name: group.Name, OwnerId: group.OwnerID, Status: groupStatusToProto(group.Status), CreatedAt: timeToProto(group.CreatedAt), UpdatedAt: timeToProto(group.UpdatedAt), CurrentUserRole: memberRoleToProto(group.CurrentUserRole)}
}
func groupStatusToProto(status domain.GroupStatus) corev1.GroupStatus {
	if status == domain.GroupArchived {
		return corev1.GroupStatus_GROUP_STATUS_ARCHIVED
	}
	return corev1.GroupStatus_GROUP_STATUS_ACTIVE
}
func memberRoleToProto(role domain.MemberRole) corev1.MemberRole {
	switch role {
	case domain.RoleOwner:
		return corev1.MemberRole_MEMBER_ROLE_OWNER
	case domain.RoleAdmin:
		return corev1.MemberRole_MEMBER_ROLE_ADMIN
	default:
		return corev1.MemberRole_MEMBER_ROLE_MEMBER
	}
}
