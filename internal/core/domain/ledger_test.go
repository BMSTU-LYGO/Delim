package domain

import "testing"

func TestCalculateBalances(t *testing.T) {
	tests := []struct {
		name  string
		input LedgerInput
		want  map[int64]int64
	}{{"two participants", LedgerInput{Expenses: []LedgerExpense{{PayerUserID: 1, AmountMinor: 100, Currency: "RUB", Status: ExpenseConfirmed, Allocations: []Allocation{{UserID: 1, AmountMinor: 50}, {UserID: 2, AmountMinor: 50}}}}}, map[int64]int64{1: 50, 2: -50}}, {"three participants", LedgerInput{Expenses: []LedgerExpense{{PayerUserID: 1, AmountMinor: 101, Currency: "RUB", Status: ExpenseConfirmed, Allocations: []Allocation{{UserID: 1, AmountMinor: 34}, {UserID: 2, AmountMinor: 34}, {UserID: 3, AmountMinor: 33}}}}}, map[int64]int64{1: 67, 2: -34, 3: -33}}, {"multiple payers", LedgerInput{Expenses: []LedgerExpense{{PayerUserID: 1, AmountMinor: 60, Currency: "RUB", Status: ExpenseConfirmed, Allocations: []Allocation{{UserID: 1, AmountMinor: 30}, {UserID: 2, AmountMinor: 30}}}, {PayerUserID: 2, AmountMinor: 40, Currency: "RUB", Status: ExpenseConfirmed, Allocations: []Allocation{{UserID: 1, AmountMinor: 20}, {UserID: 2, AmountMinor: 20}}}}}, map[int64]int64{1: 10, 2: -10}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := CalculateBalances(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			for _, balance := range got {
				if balance.NetAmountMinor != tt.want[balance.UserID] {
					t.Fatalf("got %v want %v", got, tt.want)
				}
			}
		})
	}
}
