package usecase

import (
	"context"
	"strings"

	"delim/internal/core/domain"
)

type GroupRepository interface {
	CreateGroup(context.Context, int64, string) (domain.Group, error)
	GetGroup(context.Context, int64, int64) (domain.Group, error)
	ListGroups(context.Context, int64, int64, int32) ([]domain.Group, error)
	ListGroupMembers(context.Context, int64, int64) ([]domain.GroupMember, error)
	AddGroupMembers(context.Context, int64, int64, []int64) ([]domain.GroupMember, error)
	JoinGroup(context.Context, int64, int64) (domain.GroupMember, error)
	UpdateMemberRole(context.Context, int64, int64, int64, domain.MemberRole) (domain.GroupMember, error)
	ArchiveGroup(context.Context, int64, int64) (domain.Group, error)
}

func (g *Groups) AddMembers(ctx context.Context, actorID, groupID int64, userIDs []int64) ([]domain.GroupMember, error) {
	if actorID <= 0 || groupID <= 0 {
		return nil, domain.ErrInvalidArgument
	}
	seen := make(map[int64]struct{}, len(userIDs))
	unique := make([]int64, 0, len(userIDs))
	for _, id := range userIDs {
		if id <= 0 {
			return nil, domain.ErrInvalidArgument
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}
	return g.repository.AddGroupMembers(ctx, actorID, groupID, unique)
}

func (g *Groups) ListMembers(ctx context.Context, actorID, groupID int64) ([]domain.GroupMember, error) {
	if actorID <= 0 || groupID <= 0 {
		return nil, domain.ErrInvalidArgument
	}
	return g.repository.ListGroupMembers(ctx, actorID, groupID)
}

func (g *Groups) Archive(ctx context.Context, actorID, groupID int64) (domain.Group, error) {
	if actorID <= 0 || groupID <= 0 {
		return domain.Group{}, domain.ErrInvalidArgument
	}
	return g.repository.ArchiveGroup(ctx, actorID, groupID)
}

func (g *Groups) Join(ctx context.Context, actorID, groupID int64) (domain.GroupMember, error) {
	if actorID <= 0 || groupID <= 0 {
		return domain.GroupMember{}, domain.ErrInvalidArgument
	}
	return g.repository.JoinGroup(ctx, actorID, groupID)
}

func (g *Groups) UpdateRole(ctx context.Context, actorID, groupID, userID int64, role domain.MemberRole) (domain.GroupMember, error) {
	if actorID <= 0 || groupID <= 0 || userID <= 0 || (role != domain.RoleAdmin && role != domain.RoleMember) {
		return domain.GroupMember{}, domain.ErrInvalidArgument
	}
	return g.repository.UpdateMemberRole(ctx, actorID, groupID, userID, role)
}

func (g *Groups) Get(ctx context.Context, actorID, groupID int64) (domain.Group, error) {
	if actorID <= 0 || groupID <= 0 {
		return domain.Group{}, domain.ErrInvalidArgument
	}
	return g.repository.GetGroup(ctx, actorID, groupID)
}

func (g *Groups) List(ctx context.Context, actorID, cursor int64, limit int32) ([]domain.Group, error) {
	if actorID <= 0 || cursor < 0 {
		return nil, domain.ErrInvalidArgument
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 0 || limit > 100 {
		return nil, domain.ErrInvalidArgument
	}
	return g.repository.ListGroups(ctx, actorID, cursor, limit)
}

type Groups struct{ repository GroupRepository }

func NewGroups(repository GroupRepository) *Groups { return &Groups{repository: repository} }
func (g *Groups) Create(ctx context.Context, actorID int64, name string) (domain.Group, error) {
	name = strings.TrimSpace(name)
	if actorID <= 0 || name == "" {
		return domain.Group{}, domain.ErrInvalidArgument
	}
	return g.repository.CreateGroup(ctx, actorID, name)
}
