package service

import (
	"context"
	"delim/internal/core/domain"
	corev1 "delim/pkg/gen/core/v1"
)

type GroupService interface {
	Create(context.Context, int64, domain.GroupInput) (domain.Group, error)
	BudgetSummary(context.Context, int64, int64) (domain.GroupBudgetSummary, error)
	Get(context.Context, int64, int64) (domain.Group, error)
	List(context.Context, int64, int64, int32) ([]domain.Group, error)
	Join(context.Context, int64, int64) (domain.GroupMember, error)
	ListMembers(context.Context, int64, int64) ([]domain.GroupMember, error)
	AddMembers(context.Context, int64, int64, []int64) ([]domain.GroupMember, error)
	UpdateRole(context.Context, int64, int64, int64, domain.MemberRole) (domain.GroupMember, error)
	Archive(context.Context, int64, int64) (domain.Group, error)
}

func (s *GRPCServer) ListGroupMembers(ctx context.Context, req *corev1.ListGroupMembersRequest) (*corev1.ListGroupMembersResponse, error) {
	members, err := s.groups.ListMembers(ctx, req.GetActorUserId(), req.GetGroupId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	response := &corev1.ListGroupMembersResponse{}
	for _, member := range members {
		response.Members = append(response.Members, memberToProto(member))
	}
	return response, nil
}

func (s *GRPCServer) AddGroupMembers(ctx context.Context, req *corev1.AddGroupMembersRequest) (*corev1.AddGroupMembersResponse, error) {
	members, err := s.groups.AddMembers(ctx, req.GetActorUserId(), req.GetGroupId(), req.GetUserIds())
	if err != nil {
		return nil, toGRPCError(err)
	}
	response := &corev1.AddGroupMembersResponse{}
	for _, member := range members {
		response.Members = append(response.Members, memberToProto(member))
	}
	return response, nil
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
	result := &corev1.GroupMember{GroupId: member.GroupID, UserId: member.UserID, Role: memberRoleToProto(member.Role), JoinedAt: timeToProto(member.JoinedAt)}
	if member.User.ID > 0 {
		result.User = userToProto(member.User)
	}
	return result
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
	group, err := s.groups.Create(ctx, req.GetActorUserId(), domain.GroupInput{Name: req.GetName(), ActivityType: req.GetActivityType(), Location: req.GetLocation(), StartDate: req.GetStartDate(), EndDate: req.GetEndDate(), PlannedBudgetMinor: req.PlannedBudgetMinor})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.CreateGroupResponse{Group: groupToProto(group)}, nil
}

func (s *GRPCServer) GetGroupBudgetSummary(ctx context.Context, req *corev1.GetGroupBudgetSummaryRequest) (*corev1.GetGroupBudgetSummaryResponse, error) {
	summary, err := s.groups.BudgetSummary(ctx, req.GetActorUserId(), req.GetGroupId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.GetGroupBudgetSummaryResponse{PlannedBudgetMinor: summary.PlannedBudgetMinor, ConfirmedSpendMinor: summary.ConfirmedSpendMinor, PendingSpendMinor: summary.PendingSpendMinor, TotalSpendMinor: summary.TotalSpendMinor}, nil
}

func groupToProto(group domain.Group) *corev1.Group {
	return &corev1.Group{Id: group.ID, Name: group.Name, OwnerId: group.OwnerID, Status: groupStatusToProto(group.Status), CreatedAt: timeToProto(group.CreatedAt), UpdatedAt: timeToProto(group.UpdatedAt), CurrentUserRole: memberRoleToProto(group.CurrentUserRole), ActivityType: group.ActivityType, Location: group.Location, StartDate: group.StartDate, EndDate: group.EndDate, PlannedBudgetMinor: group.PlannedBudgetMinor}
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
