package postgres

import (
	"context"
	"delim/internal/core/domain"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *Store) ensureMember(ctx context.Context, actorID, groupID int64) error {
	var found int64
	err := s.pool.QueryRow(ctx, `SELECT user_id FROM group_members WHERE group_id=$1 AND user_id=$2`, groupID, actorID).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrForbidden
	}
	return err
}

func (s *Store) LoadLedger(ctx context.Context, actorID, groupID int64) (domain.LedgerInput, error) {
	if err := s.ensureMember(ctx, actorID, groupID); err != nil {
		return domain.LedgerInput{}, err
	}
	var input domain.LedgerInput
	expenseRows, err := s.pool.Query(ctx, `SELECT e.id,e.payer_user_id,e.amount_minor,e.currency,e.status,e.created_at,a.id,a.user_id,a.amount_minor FROM expenses e LEFT JOIN allocations a ON a.expense_id=e.id WHERE e.group_id=$1 AND e.status='confirmed' ORDER BY e.id,a.id`, groupID)
	if err != nil {
		return domain.LedgerInput{}, err
	}
	expenseIndexes := map[int64]int{}
	for expenseRows.Next() {
		var expense domain.LedgerExpense
		var allocationID, userID, allocationAmount *int64
		if err := expenseRows.Scan(&expense.ID, &expense.PayerUserID, &expense.AmountMinor, &expense.Currency, &expense.Status, &expense.CreatedAt, &allocationID, &userID, &allocationAmount); err != nil {
			expenseRows.Close()
			return domain.LedgerInput{}, err
		}
		index, ok := expenseIndexes[expense.ID]
		if !ok {
			index = len(input.Expenses)
			expenseIndexes[expense.ID] = index
			input.Expenses = append(input.Expenses, expense)
		}
		if allocationID != nil {
			input.Expenses[index].Allocations = append(input.Expenses[index].Allocations, domain.Allocation{ID: *allocationID, ExpenseID: expense.ID, UserID: *userID, AmountMinor: *allocationAmount})
		}
	}
	if err := expenseRows.Err(); err != nil {
		expenseRows.Close()
		return domain.LedgerInput{}, err
	}
	expenseRows.Close()
	settlementRows, err := s.pool.Query(ctx, `SELECT id,sender_user_id,receiver_user_id,amount_minor,currency,status,created_at FROM settlements WHERE group_id=$1 AND status='confirmed' ORDER BY id`, groupID)
	if err != nil {
		return domain.LedgerInput{}, err
	}
	for settlementRows.Next() {
		var value domain.LedgerSettlement
		if err := settlementRows.Scan(&value.ID, &value.SenderUserID, &value.ReceiverUserID, &value.AmountMinor, &value.Currency, &value.Status, &value.CreatedAt); err != nil {
			settlementRows.Close()
			return domain.LedgerInput{}, err
		}
		input.Settlements = append(input.Settlements, value)
	}
	if err := settlementRows.Err(); err != nil {
		settlementRows.Close()
		return domain.LedgerInput{}, err
	}
	settlementRows.Close()
	adjustmentRows, err := s.pool.Query(ctx, `SELECT a.id,e.payer_user_id,a.amount_minor,a.currency,a.type,a.created_at,aa.user_id,aa.amount_minor FROM adjustments a JOIN expenses e ON e.id=a.expense_id LEFT JOIN adjustment_allocations aa ON aa.adjustment_id=a.id WHERE a.group_id=$1 ORDER BY a.id,aa.user_id`, groupID)
	if err != nil {
		return domain.LedgerInput{}, err
	}
	defer adjustmentRows.Close()
	adjustmentIndexes := map[int64]int{}
	for adjustmentRows.Next() {
		var value domain.LedgerAdjustment
		var userID, amount *int64
		if err := adjustmentRows.Scan(&value.ID, &value.PayerUserID, &value.AmountMinor, &value.Currency, &value.Type, &value.CreatedAt, &userID, &amount); err != nil {
			return domain.LedgerInput{}, err
		}
		index, ok := adjustmentIndexes[value.ID]
		if !ok {
			index = len(input.Adjustments)
			adjustmentIndexes[value.ID] = index
			input.Adjustments = append(input.Adjustments, value)
		}
		if userID != nil {
			input.Adjustments[index].Allocations = append(input.Adjustments[index].Allocations, domain.AdjustmentAllocation{UserID: *userID, AmountMinor: *amount})
		}
	}
	if err := adjustmentRows.Err(); err != nil {
		return domain.LedgerInput{}, err
	}
	return input, nil
}
