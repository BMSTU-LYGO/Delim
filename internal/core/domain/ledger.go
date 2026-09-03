package domain

import "time"

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
