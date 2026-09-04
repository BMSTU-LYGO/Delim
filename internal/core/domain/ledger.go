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
		if expense.AmountMinor <= 0 || expense.Currency == "" {
			return nil, ErrInvalidArgument
		}
		var allocated int64
		for _, allocation := range expense.Allocations {
			if allocation.AmountMinor < 0 || allocated > math.MaxInt64-allocation.AmountMinor {
				return nil, ErrInvalidArgument
			}
			allocated += allocation.AmountMinor
		}
		if allocated != expense.AmountMinor {
			return nil, ErrInvalidArgument
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
		if settlement.AmountMinor <= 0 || settlement.Currency == "" || settlement.SenderUserID == settlement.ReceiverUserID {
			return nil, ErrInvalidArgument
		}
		if err := add(settlement.SenderUserID, settlement.Currency, settlement.AmountMinor); err != nil {
			return nil, err
		}
		if err := add(settlement.ReceiverUserID, settlement.Currency, -settlement.AmountMinor); err != nil {
			return nil, err
		}
	}
	for _, adjustment := range input.Adjustments {
		if adjustment.AmountMinor <= 0 || adjustment.Currency == "" || (adjustment.Type != AdjustmentRefund && adjustment.Type != AdjustmentCorrection) {
			return nil, ErrInvalidArgument
		}
		var allocated int64
		for _, allocation := range adjustment.Allocations {
			if allocation.AmountMinor < 0 || allocated > math.MaxInt64-allocation.AmountMinor {
				return nil, ErrInvalidArgument
			}
			allocated += allocation.AmountMinor
		}
		if allocated != adjustment.AmountMinor {
			return nil, ErrInvalidArgument
		}
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
	currencyTotals := make(map[string]int64)
	for k, amount := range values {
		balances = append(balances, Balance{UserID: k.userID, Currency: k.currency, NetAmountMinor: amount})
		current := currencyTotals[k.currency]
		if (amount > 0 && current > math.MaxInt64-amount) || (amount < 0 && current < math.MinInt64-amount) {
			return nil, ErrInvalidArgument
		}
		currencyTotals[k.currency] = current + amount
	}
	for _, total := range currencyTotals {
		if total != 0 {
			return nil, ErrInvalidArgument
		}
	}
	sort.Slice(balances, func(i, j int) bool {
		if balances[i].Currency != balances[j].Currency {
			return balances[i].Currency < balances[j].Currency
		}
		return balances[i].UserID < balances[j].UserID
	})
	return balances, nil
}

func CalculateBalanceBreakdown(input LedgerInput, userID int64) ([]BalanceEntry, error) {
	if _, err := CalculateBalances(input); err != nil {
		return nil, err
	}
	var entries []BalanceEntry
	appendEntry := func(operationType string, operationID int64, currency string, amount int64, occurredAt time.Time) {
		if amount != 0 {
			entries = append(entries, BalanceEntry{OperationType: operationType, OperationID: operationID, Currency: currency, AmountMinor: amount, OccurredAt: occurredAt})
		}
	}
	for _, expense := range input.Expenses {
		if expense.Status != ExpenseConfirmed {
			continue
		}
		if expense.PayerUserID == userID {
			appendEntry("expense", expense.ID, expense.Currency, expense.AmountMinor, expense.CreatedAt)
		}
		for _, allocation := range expense.Allocations {
			if allocation.UserID == userID {
				appendEntry("allocation", expense.ID, expense.Currency, -allocation.AmountMinor, expense.CreatedAt)
			}
		}
	}
	for _, settlement := range input.Settlements {
		if settlement.Status != SettlementConfirmed {
			continue
		}
		if settlement.SenderUserID == userID {
			appendEntry("settlement_sent", settlement.ID, settlement.Currency, settlement.AmountMinor, settlement.CreatedAt)
		}
		if settlement.ReceiverUserID == userID {
			appendEntry("settlement_received", settlement.ID, settlement.Currency, -settlement.AmountMinor, settlement.CreatedAt)
		}
	}
	for _, adjustment := range input.Adjustments {
		sign := int64(1)
		if adjustment.Type == AdjustmentRefund {
			sign = -1
		}
		if adjustment.PayerUserID == userID {
			appendEntry("adjustment_payer", adjustment.ID, adjustment.Currency, sign*adjustment.AmountMinor, adjustment.CreatedAt)
		}
		for _, allocation := range adjustment.Allocations {
			if allocation.UserID == userID {
				appendEntry("adjustment_allocation", adjustment.ID, adjustment.Currency, -sign*allocation.AmountMinor, adjustment.CreatedAt)
			}
		}
	}
	sort.SliceStable(entries, func(i, j int) bool {
		if !entries[i].OccurredAt.Equal(entries[j].OccurredAt) {
			return entries[i].OccurredAt.Before(entries[j].OccurredAt)
		}
		if entries[i].OperationType != entries[j].OperationType {
			return entries[i].OperationType < entries[j].OperationType
		}
		return entries[i].OperationID < entries[j].OperationID
	})
	return entries, nil
}

func PlanSettlements(balances []Balance) ([]SettlementPlanTransfer, error) {
	byCurrency := make(map[string][]Balance)
	for _, balance := range balances {
		if balance.NetAmountMinor != 0 {
			byCurrency[balance.Currency] = append(byCurrency[balance.Currency], balance)
		}
	}
	currencies := make([]string, 0, len(byCurrency))
	for currency := range byCurrency {
		currencies = append(currencies, currency)
	}
	sort.Strings(currencies)
	var transfers []SettlementPlanTransfer
	for _, currency := range currencies {
		var debtors, creditors []Balance
		var total int64
		for _, balance := range byCurrency[currency] {
			if balance.NetAmountMinor == math.MinInt64 {
				return nil, ErrInvalidArgument
			}
			if (balance.NetAmountMinor > 0 && total > math.MaxInt64-balance.NetAmountMinor) || (balance.NetAmountMinor < 0 && total < math.MinInt64-balance.NetAmountMinor) {
				return nil, ErrInvalidArgument
			}
			total += balance.NetAmountMinor
			if balance.NetAmountMinor < 0 {
				debtors = append(debtors, balance)
			} else {
				creditors = append(creditors, balance)
			}
		}
		if total != 0 {
			return nil, ErrInvalidArgument
		}
		sort.Slice(debtors, func(i, j int) bool { return debtors[i].UserID < debtors[j].UserID })
		sort.Slice(creditors, func(i, j int) bool { return creditors[i].UserID < creditors[j].UserID })
		di, ci := 0, 0
		for di < len(debtors) && ci < len(creditors) {
			debt := -debtors[di].NetAmountMinor
			credit := creditors[ci].NetAmountMinor
			amount := debt
			if credit < amount {
				amount = credit
			}
			if amount > 0 {
				transfers = append(transfers, SettlementPlanTransfer{FromUserID: debtors[di].UserID, ToUserID: creditors[ci].UserID, AmountMinor: amount, Currency: currency})
			}
			debtors[di].NetAmountMinor += amount
			creditors[ci].NetAmountMinor -= amount
			if debtors[di].NetAmountMinor == 0 {
				di++
			}
			if creditors[ci].NetAmountMinor == 0 {
				ci++
			}
		}
	}
	return transfers, nil
}
