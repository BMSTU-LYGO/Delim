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
