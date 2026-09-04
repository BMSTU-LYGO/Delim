package domain

import "time"

type GroupStatus string

const (
	GroupActive   GroupStatus = "active"
	GroupArchived GroupStatus = "archived"
)

type MemberRole string

const (
	RoleOwner  MemberRole = "owner"
	RoleAdmin  MemberRole = "admin"
	RoleMember MemberRole = "member"
)

type Group struct {
	ID                   int64
	Name                 string
	OwnerID              int64
	Status               GroupStatus
	CreatedAt, UpdatedAt time.Time
	CurrentUserRole      MemberRole
}
type GroupMember struct {
	GroupID, UserID int64
	Role            MemberRole
	JoinedAt        time.Time
	User            User
}

func ValidateMemberAdd(status GroupStatus, actorRole MemberRole) error {
	if status == GroupArchived {
		return ErrArchivedGroup
	}
	if actorRole != RoleOwner && actorRole != RoleAdmin {
		return ErrForbidden
	}
	return nil
}

func ValidateRoleChange(status GroupStatus, actorRole, targetRole, desiredRole MemberRole) error {
	if status == GroupArchived {
		return ErrArchivedGroup
	}
	if desiredRole != RoleAdmin && desiredRole != RoleMember {
		return ErrInvalidArgument
	}
	if actorRole != RoleOwner || targetRole == RoleOwner {
		return ErrForbidden
	}
	return nil
}
