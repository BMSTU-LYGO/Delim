package service

import (
	"context"
	"delim/internal/core/domain"
	corev1 "delim/pkg/gen/core/v1"
)

type GroupService interface {
	Create(context.Context, int64, string) (domain.Group, error)
}

func (s *GRPCServer) CreateGroup(ctx context.Context, req *corev1.CreateGroupRequest) (*corev1.CreateGroupResponse, error) {
	group, err := s.groups.Create(ctx, req.GetActorUserId(), req.GetName())
	if err != nil {
		return nil, err
	}
	return &corev1.CreateGroupResponse{Group: groupToProto(group)}, nil
}

func groupToProto(group domain.Group) *corev1.Group {
	return &corev1.Group{Id: group.ID, Name: group.Name, OwnerId: group.OwnerID, Status: groupStatusToProto(group.Status), CreatedAtUnix: group.CreatedAt.Unix(), UpdatedAtUnix: group.UpdatedAt.Unix(), CurrentUserRole: memberRoleToProto(group.CurrentUserRole)}
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
