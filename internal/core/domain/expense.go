package domain

import "time"

type SplitType string

const (
	SplitEqual      SplitType = "equal"
	SplitFixed      SplitType = "fixed"
	SplitShares     SplitType = "shares"
	SplitPercentage SplitType = "percentage"
	SplitItem       SplitType = "item"
)

type ExpenseStatus string

const (
	ExpensePending   ExpenseStatus = "pending"
	ExpenseConfirmed ExpenseStatus = "confirmed"
	ExpenseCancelled ExpenseStatus = "cancelled"
)

type SplitParticipant struct{ UserID, Value int64 }
type ExpenseItemInput struct {
	Name               string
	AmountMinor        int64
	ParticipantUserIDs []int64
}
type ExpenseInput struct {
	GroupID, PayerUserID, AmountMinor int64
	Currency, Description             string
	ExpenseDate                       time.Time
	SplitType                         SplitType
	Participants                      []SplitParticipant
	Items                             []ExpenseItemInput
}
type ExpenseItem struct {
	ID, ExpenseID int64
	Name          string
	AmountMinor   int64
	Position      int32
}
type Allocation struct {
	ID, ExpenseID       int64
	ExpenseItemID       *int64
	UserID, AmountMinor int64
}
type AllocationDraft struct {
	ItemIndex   *int
	UserID      int64
	AmountMinor int64
}
type Expense struct {
	ID, GroupID, PayerUserID, CreatedBy, AmountMinor int64
	Currency, Description                            string
	ExpenseDate                                      time.Time
	SplitType                                        SplitType
	Status                                           ExpenseStatus
	Version                                          int64
	CreatedAt, UpdatedAt                             time.Time
	Items                                            []ExpenseItem
	Allocations                                      []Allocation
}
