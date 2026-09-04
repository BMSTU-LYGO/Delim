package domain

import (
	"errors"
	"testing"
)

func TestGroupPermissionMatrix(t *testing.T) {
	tests := []struct {
		name, operation        string
		status                 GroupStatus
		actor, target, desired MemberRole
		want                   error
	}{
		{"owner adds", "add", GroupActive, RoleOwner, "", "", nil},
		{"admin adds", "add", GroupActive, RoleAdmin, "", "", nil},
		{"member cannot add", "add", GroupActive, RoleMember, "", "", ErrForbidden},
		{"archived owner cannot add", "add", GroupArchived, RoleOwner, "", "", ErrArchivedGroup},
		{"owner promotes member", "role", GroupActive, RoleOwner, RoleMember, RoleAdmin, nil},
		{"admin cannot change role", "role", GroupActive, RoleAdmin, RoleMember, RoleAdmin, ErrForbidden},
		{"owner remains owner", "role", GroupActive, RoleOwner, RoleOwner, RoleMember, ErrForbidden},
		{"archived role change", "role", GroupArchived, RoleOwner, RoleMember, RoleAdmin, ErrArchivedGroup},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got error
			if tt.operation == "add" {
				got = ValidateMemberAdd(tt.status, tt.actor)
			} else {
				got = ValidateRoleChange(tt.status, tt.actor, tt.target, tt.desired)
			}
			if !errors.Is(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
