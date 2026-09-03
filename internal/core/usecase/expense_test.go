package usecase

import (
	"delim/internal/core/domain"
	"errors"
	"testing"
)

func TestValidateExpenseUpdateVersionConflict(t *testing.T) {
	tests := []struct {
		name             string
		current          domain.Expense
		version, groupID int64
		want             error
	}{{"valid", domain.Expense{GroupID: 1, Status: domain.ExpensePending, Version: 2}, 2, 1, nil}, {"stale", domain.Expense{GroupID: 1, Status: domain.ExpensePending, Version: 3}, 2, 1, domain.ErrConflict}, {"confirmed", domain.Expense{GroupID: 1, Status: domain.ExpenseConfirmed, Version: 2}, 2, 1, domain.ErrInvalidState}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateExpenseUpdate(tt.current, tt.version, tt.groupID); !errors.Is(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
