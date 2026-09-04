package domain

import (
	"fmt"
	"testing"
)

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

func TestCalculateBalancesFinancialInvariants(t *testing.T) {
	input := LedgerInput{
		Expenses:    []LedgerExpense{{PayerUserID: 1, AmountMinor: 100, Currency: "RUB", Status: ExpenseConfirmed, Allocations: []Allocation{{UserID: 1, AmountMinor: 50}, {UserID: 2, AmountMinor: 50}}}, {PayerUserID: 2, AmountMinor: 999, Currency: "USD", Status: ExpensePending, Allocations: []Allocation{{UserID: 1, AmountMinor: 999}}}},
		Settlements: []LedgerSettlement{{SenderUserID: 2, ReceiverUserID: 1, AmountMinor: 20, Currency: "RUB", Status: SettlementConfirmed}, {SenderUserID: 2, ReceiverUserID: 1, AmountMinor: 50, Currency: "RUB", Status: SettlementPending}},
		Adjustments: []LedgerAdjustment{{PayerUserID: 1, AmountMinor: 40, Currency: "RUB", Type: AdjustmentRefund, Allocations: []AdjustmentAllocation{{UserID: 1, AmountMinor: 20}, {UserID: 2, AmountMinor: 20}}}},
	}
	balances, err := CalculateBalances(input)
	if err != nil {
		t.Fatal(err)
	}
	want := map[int64]int64{1: 10, 2: -10}
	totals := map[string]int64{}
	for _, balance := range balances {
		totals[balance.Currency] += balance.NetAmountMinor
		if balance.Currency == "RUB" && balance.NetAmountMinor != want[balance.UserID] {
			t.Fatalf("got %v want %v", balances, want)
		}
		if balance.Currency == "USD" && balance.NetAmountMinor != 0 {
			t.Fatalf("pending expense affected USD: %v", balances)
		}
	}
	for currency, total := range totals {
		if total != 0 {
			t.Fatalf("%s total is %d", currency, total)
		}
	}
}

func TestLedgerOperationMatrix(t *testing.T) {
	base := LedgerExpense{PayerUserID: 1, AmountMinor: 100, Currency: "RUB", Status: ExpenseConfirmed, Allocations: []Allocation{{UserID: 1, AmountMinor: 50}, {UserID: 2, AmountMinor: 50}}}
	tests := []struct {
		name  string
		input LedgerInput
		want  map[string]int64
	}{{"one expense", LedgerInput{Expenses: []LedgerExpense{base}}, map[string]int64{"RUB/1": 50, "RUB/2": -50}}, {"multiple currencies", LedgerInput{Expenses: []LedgerExpense{base, {PayerUserID: 2, AmountMinor: 80, Currency: "USD", Status: ExpenseConfirmed, Allocations: []Allocation{{UserID: 1, AmountMinor: 40}, {UserID: 2, AmountMinor: 40}}}}}, map[string]int64{"RUB/1": 50, "RUB/2": -50, "USD/1": -40, "USD/2": 40}}, {"confirmed settlement", LedgerInput{Expenses: []LedgerExpense{base}, Settlements: []LedgerSettlement{{SenderUserID: 2, ReceiverUserID: 1, AmountMinor: 20, Currency: "RUB", Status: SettlementConfirmed}}}, map[string]int64{"RUB/1": 30, "RUB/2": -30}}, {"refund", LedgerInput{Expenses: []LedgerExpense{base}, Adjustments: []LedgerAdjustment{{PayerUserID: 1, AmountMinor: 40, Currency: "RUB", Type: AdjustmentRefund, Allocations: []AdjustmentAllocation{{UserID: 1, AmountMinor: 20}, {UserID: 2, AmountMinor: 20}}}}}, map[string]int64{"RUB/1": 30, "RUB/2": -30}}, {"correction", LedgerInput{Expenses: []LedgerExpense{base}, Adjustments: []LedgerAdjustment{{PayerUserID: 1, AmountMinor: 40, Currency: "RUB", Type: AdjustmentCorrection, Allocations: []AdjustmentAllocation{{UserID: 1, AmountMinor: 20}, {UserID: 2, AmountMinor: 20}}}}}, map[string]int64{"RUB/1": 70, "RUB/2": -70}}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			balances, err := CalculateBalances(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			if len(balances) != len(tt.want) {
				t.Fatalf("got %v want %v", balances, tt.want)
			}
			totals := map[string]int64{}
			for _, balance := range balances {
				key := fmt.Sprintf("%s/%d", balance.Currency, balance.UserID)
				if balance.NetAmountMinor != tt.want[key] {
					t.Fatalf("got %v want %v", balances, tt.want)
				}
				totals[balance.Currency] += balance.NetAmountMinor
			}
			for currency, total := range totals {
				if total != 0 {
					t.Fatalf("%s total %d", currency, total)
				}
			}
		})
	}
}

func TestLedgerRejectsUnbalancedOperation(t *testing.T) {
	_, err := CalculateBalances(LedgerInput{Expenses: []LedgerExpense{{PayerUserID: 1, AmountMinor: 100, Currency: "RUB", Status: ExpenseConfirmed, Allocations: []Allocation{{UserID: 2, AmountMinor: 99}}}}})
	if err == nil {
		t.Fatal("expected error")
	}
}
