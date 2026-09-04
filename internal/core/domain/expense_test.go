package domain

import (
	"errors"
	"testing"
)

func TestExpenseStateTransitions(t *testing.T) {
	tests := []struct {
		name            string
		current, target ExpenseStatus
		want            error
	}{{"confirm pending", ExpensePending, ExpenseConfirmed, nil}, {"cancel pending", ExpensePending, ExpenseCancelled, nil}, {"confirm idempotent", ExpenseConfirmed, ExpenseConfirmed, nil}, {"cancel idempotent", ExpenseCancelled, ExpenseCancelled, nil}, {"cancel confirmed", ExpenseConfirmed, ExpenseCancelled, ErrInvalidState}, {"confirm cancelled", ExpenseCancelled, ExpenseConfirmed, ErrInvalidState}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ValidateExpenseTransition(tt.current, tt.target); !errors.Is(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
