package domain

import (
	"math"
	"sort"
	"time"
)

type Balance struct {
	UserID         int64
	Currency       string
	NetAmountMinor int64
}
type BalanceEntry struct {
	OperationType string
	OperationID   int64
	Currency      string
	AmountMinor   int64
	OccurredAt    time.Time
}
type SettlementPlanTransfer struct {
	FromUserID, ToUserID, AmountMinor int64
	Currency                          string
}

type LedgerExpense struct {
	ID, PayerUserID, AmountMinor int64
	Currency                     string
	Status                       ExpenseStatus
	Allocations                  []Allocation
	CreatedAt                    time.Time
}
type LedgerSettlement struct {
	ID, SenderUserID, ReceiverUserID, AmountMinor int64
	Currency                                      string
	Status                                        SettlementStatus
	CreatedAt                                     time.Time
}
type LedgerAdjustment struct {
	ID, PayerUserID, AmountMinor int64
	Currency                     string
	Type                         AdjustmentType
	Allocations                  []AdjustmentAllocation
	CreatedAt                    time.Time
}
type LedgerInput struct {
	Expenses    []LedgerExpense
	Settlements []LedgerSettlement
	Adjustments []LedgerAdjustment
}

func CalculateBalances(input LedgerInput) ([]Balance, error) {
	type key struct {
		userID   int64
		currency string
	}
	values := make(map[key]int64)
	add := func(userID int64, currency string, amount int64) error {
		k := key{userID, currency}
		current := values[k]
		if (amount > 0 && current > math.MaxInt64-amount) || (amount < 0 && current < math.MinInt64-amount) {
			return ErrInvalidArgument
		}
		values[k] = current + amount
		return nil
	}
	for _, expense := range input.Expenses {
		if expense.Status != ExpenseConfirmed {
			continue
		}
		if err := add(expense.PayerUserID, expense.Currency, expense.AmountMinor); err != nil {
			return nil, err
		}
		for _, allocation := range expense.Allocations {
			if err := add(allocation.UserID, expense.Currency, -allocation.AmountMinor); err != nil {
				return nil, err
			}
		}
	}
	for _, settlement := range input.Settlements {
		if settlement.Status != SettlementConfirmed {
			continue
		}
		if err := add(settlement.SenderUserID, settlement.Currency, settlement.AmountMinor); err != nil {
			return nil, err
		}
		if err := add(settlement.ReceiverUserID, settlement.Currency, -settlement.AmountMinor); err != nil {
			return nil, err
		}
	}
	for _, adjustment := range input.Adjustments {
		sign := int64(1)
		if adjustment.Type == AdjustmentRefund {
			sign = -1
		}
		if err := add(adjustment.PayerUserID, adjustment.Currency, sign*adjustment.AmountMinor); err != nil {
			return nil, err
		}
		for _, allocation := range adjustment.Allocations {
			if err := add(allocation.UserID, adjustment.Currency, -sign*allocation.AmountMinor); err != nil {
				return nil, err
			}
		}
	}
	balances := make([]Balance, 0, len(values))
	for k, amount := range values {
		balances = append(balances, Balance{UserID: k.userID, Currency: k.currency, NetAmountMinor: amount})
	}
	sort.Slice(balances, func(i, j int) bool {
		if balances[i].Currency != balances[j].Currency {
			return balances[i].Currency < balances[j].Currency
		}
		return balances[i].UserID < balances[j].UserID
	})
	return balances, nil
}
