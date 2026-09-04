package domain

import (
	"errors"
	"testing"
)

func TestAdjustmentPermission(t *testing.T) {
	tests := []struct {
		name           string
		actor, creator int64
		role           MemberRole
		want           error
	}{{"expense creator", 2, 2, RoleMember, nil}, {"owner", 1, 2, RoleOwner, nil}, {"admin", 3, 2, RoleAdmin, nil}, {"unrelated member", 4, 2, RoleMember, ErrForbidden}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateAdjustmentPermission(tt.actor, tt.creator, tt.role); !errors.Is(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
