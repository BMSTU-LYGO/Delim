package domain

import "time"

type AdjustmentType string

const (
	// AdjustmentRefund reduces the payer contribution and compensates allocations
	// of the original expense. Use it for every downward correction.
	AdjustmentRefund AdjustmentType = "refund"
	// AdjustmentCorrection adds a positive contribution to the original expense.
	AdjustmentCorrection AdjustmentType = "correction"
)

type AdjustmentAllocation struct{ UserID, AmountMinor int64 }
type Adjustment struct {
	ID, GroupID, ExpenseID int64
	Type                   AdjustmentType
	AmountMinor            int64
	Currency               string
	CreatedBy              int64
	CreatedAt              time.Time
	Allocations            []AdjustmentAllocation
}
