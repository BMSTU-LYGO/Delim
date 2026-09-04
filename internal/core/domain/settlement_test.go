package domain

import (
	"errors"
	"testing"
)

func TestSettlementConfirmationState(t *testing.T) {
	tests := []struct {
		name   string
		actor  int64
		status SettlementStatus
		want   error
	}{{"receiver confirms", 2, SettlementPending, nil}, {"confirmed idempotent", 2, SettlementConfirmed, nil}, {"sender rejected", 1, SettlementPending, ErrForbidden}, {"cancelled rejected", 2, SettlementCancelled, ErrInvalidState}, {"unknown rejected", 2, "unknown", ErrInvalidState}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateSettlementConfirmation(tt.actor, 2, tt.status); !errors.Is(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
