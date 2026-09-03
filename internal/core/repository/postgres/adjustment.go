package postgres

import (
	"context"
	"delim/internal/core/domain"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateAdjustment(ctx context.Context, actorID int64, value domain.Adjustment) (domain.Adjustment, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Adjustment{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var expenseStatus domain.ExpenseStatus
	var expenseAmount int64
	var groupStatus domain.GroupStatus
	var expenseCurrency string
	err = tx.QueryRow(ctx, `SELECT e.group_id,e.status,e.amount_minor,e.currency,g.status FROM expenses e JOIN groups g ON g.id=e.group_id WHERE e.id=$1 FOR UPDATE OF e,g`, value.ExpenseID).Scan(&value.GroupID, &expenseStatus, &expenseAmount, &expenseCurrency, &groupStatus)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Adjustment{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Adjustment{}, err
	}
	if value.Currency != expenseCurrency {
		return domain.Adjustment{}, domain.ErrInvalidArgument
	}
	if groupStatus == domain.GroupArchived {
		return domain.Adjustment{}, domain.ErrArchivedGroup
	}
	if expenseStatus != domain.ExpenseConfirmed {
		return domain.Adjustment{}, domain.ErrInvalidState
	}
	var members int
	err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM group_members WHERE group_id=$1 AND user_id=$2`, value.GroupID, actorID).Scan(&members)
	if err != nil {
		return domain.Adjustment{}, err
	}
	if members != 1 {
		return domain.Adjustment{}, domain.ErrForbidden
	}
	memberRows, err := tx.Query(ctx, `SELECT user_id FROM group_members WHERE group_id=$1`, value.GroupID)
	if err != nil {
		return domain.Adjustment{}, err
	}
	memberSet := map[int64]struct{}{}
	for memberRows.Next() {
		var id int64
		if err := memberRows.Scan(&id); err != nil {
			memberRows.Close()
			return domain.Adjustment{}, err
		}
		memberSet[id] = struct{}{}
	}
	if err := memberRows.Err(); err != nil {
		memberRows.Close()
		return domain.Adjustment{}, err
	}
	memberRows.Close()
	var total int64
	seen := map[int64]struct{}{}
	for _, allocation := range value.Allocations {
		if allocation.AmountMinor < 0 || allocation.AmountMinor > value.AmountMinor-total {
			return domain.Adjustment{}, domain.ErrInvalidArgument
		}
		if _, ok := memberSet[allocation.UserID]; !ok {
			return domain.Adjustment{}, domain.ErrInvalidArgument
		}
		if _, ok := seen[allocation.UserID]; ok {
			return domain.Adjustment{}, domain.ErrInvalidArgument
		}
		seen[allocation.UserID] = struct{}{}
		total += allocation.AmountMinor
	}
	if total != value.AmountMinor {
		return domain.Adjustment{}, domain.ErrInvalidArgument
	}
	if value.Type == domain.AdjustmentRefund {
		var refunded int64
		if err := tx.QueryRow(ctx, `SELECT COALESCE(SUM(amount_minor),0)::bigint FROM adjustments WHERE expense_id=$1 AND type='refund'`, value.ExpenseID).Scan(&refunded); err != nil {
			return domain.Adjustment{}, err
		}
		if value.AmountMinor > expenseAmount-refunded {
			return domain.Adjustment{}, domain.ErrInvalidArgument
		}
	}
	value.CreatedBy = actorID
	err = tx.QueryRow(ctx, `INSERT INTO adjustments(group_id,expense_id,type,amount_minor,currency,created_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,created_at`, value.GroupID, value.ExpenseID, value.Type, value.AmountMinor, value.Currency, actorID).Scan(&value.ID, &value.CreatedAt)
	if err != nil {
		return domain.Adjustment{}, err
	}
	for _, allocation := range value.Allocations {
		if _, err = tx.Exec(ctx, `INSERT INTO adjustment_allocations(adjustment_id,user_id,amount_minor) VALUES($1,$2,$3)`, value.ID, allocation.UserID, allocation.AmountMinor); err != nil {
			return domain.Adjustment{}, err
		}
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(group_id,actor_user_id,action,entity_type,entity_id,metadata) VALUES($1,$2,'adjustment.created','adjustment',$3,jsonb_build_object('type',$4::text))`, value.GroupID, actorID, value.ID, value.Type); err != nil {
		return domain.Adjustment{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Adjustment{}, err
	}
	return value, nil
}

func (s *Store) ListAdjustments(ctx context.Context, actorID, expenseID int64) ([]domain.Adjustment, error) {
	var groupID int64
	if err := s.pool.QueryRow(ctx, `SELECT e.group_id FROM expenses e JOIN group_members gm ON gm.group_id=e.group_id AND gm.user_id=$1 WHERE e.id=$2`, actorID, expenseID).Scan(&groupID); errors.Is(err, pgx.ErrNoRows) {
		return nil, domain.ErrNotFound
	} else if err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT a.id,a.group_id,a.expense_id,a.type,a.amount_minor,a.currency,a.created_by,a.created_at,aa.user_id,aa.amount_minor FROM adjustments a JOIN expenses e ON e.id=a.expense_id JOIN group_members gm ON gm.group_id=e.group_id AND gm.user_id=$1 LEFT JOIN adjustment_allocations aa ON aa.adjustment_id=a.id WHERE a.expense_id=$2 ORDER BY a.id,aa.user_id`, actorID, expenseID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.Adjustment
	byID := map[int64]int{}
	for rows.Next() {
		var value domain.Adjustment
		var userID, amount *int64
		if err := rows.Scan(&value.ID, &value.GroupID, &value.ExpenseID, &value.Type, &value.AmountMinor, &value.Currency, &value.CreatedBy, &value.CreatedAt, &userID, &amount); err != nil {
			return nil, err
		}
		index, ok := byID[value.ID]
		if !ok {
			index = len(result)
			byID[value.ID] = index
			result = append(result, value)
		}
		if userID != nil {
			result[index].Allocations = append(result[index].Allocations, domain.AdjustmentAllocation{UserID: *userID, AmountMinor: *amount})
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return result, nil
}
