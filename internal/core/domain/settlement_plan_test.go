package domain

import "testing"

func TestPlanSettlements(t *testing.T) {
	balances := []Balance{{1, "USD", -10}, {2, "RUB", -70}, {1, "RUB", -30}, {4, "RUB", 100}, {3, "USD", 10}}
	transfers, err := PlanSettlements(balances)
	if err != nil {
		t.Fatal(err)
	}
	want := []SettlementPlanTransfer{{1, 4, 30, "RUB"}, {2, 4, 70, "RUB"}, {1, 3, 10, "USD"}}
	if len(transfers) != len(want) {
		t.Fatalf("got %v want %v", transfers, want)
	}
	for i := range want {
		if transfers[i] != want[i] {
			t.Fatalf("got %v want %v", transfers, want)
		}
	}
	net := make(map[string]map[int64]int64)
	for _, balance := range balances {
		if net[balance.Currency] == nil {
			net[balance.Currency] = map[int64]int64{}
		}
		net[balance.Currency][balance.UserID] = balance.NetAmountMinor
	}
	for _, transfer := range transfers {
		net[transfer.Currency][transfer.FromUserID] += transfer.AmountMinor
		net[transfer.Currency][transfer.ToUserID] -= transfer.AmountMinor
	}
	for currency, values := range net {
		for user, amount := range values {
			if amount != 0 {
				t.Fatalf("%s user %d remains %d", currency, user, amount)
			}
		}
	}
}
func TestPlanSettlementsRejectsUnbalancedCurrency(t *testing.T) {
	if _, err := PlanSettlements([]Balance{{1, "RUB", 1}, {2, "USD", -1}}); err == nil {
		t.Fatal("expected error")
	}
}
