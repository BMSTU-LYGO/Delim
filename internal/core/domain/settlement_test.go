package domain

import (
	"errors"
	"testing"
)

func TestSettlementConfirmationState(t *testing.T) {
	tests := []struct {
		name   string
		status SettlementStatus
		want   error
	}{{"pending", SettlementPending, nil}, {"confirmed idempotent", SettlementConfirmed, nil}, {"cancelled rejected", SettlementCancelled, ErrInvalidState}, {"unknown rejected", "unknown", ErrInvalidState}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateSettlementConfirmation(tt.status); !errors.Is(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
