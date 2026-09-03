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

func (s *Store) GetBalance(ctx context.Context, actorID, groupID int64) ([]domain.Balance, error) {
	if err := s.ensureMember(ctx, actorID, groupID); err != nil {
		return nil, err
	}
	rows, err := s.pool.Query(ctx, `WITH entries AS (
		SELECT e.payer_user_id user_id,e.currency,e.amount_minor amount FROM expenses e WHERE e.group_id=$1 AND e.status='confirmed'
		UNION ALL SELECT a.user_id,e.currency,-a.amount_minor FROM allocations a JOIN expenses e ON e.id=a.expense_id WHERE e.group_id=$1 AND e.status='confirmed'
		UNION ALL SELECT s.sender_user_id,s.currency,s.amount_minor FROM settlements s WHERE s.group_id=$1 AND s.status='confirmed'
		UNION ALL SELECT s.receiver_user_id,s.currency,-s.amount_minor FROM settlements s WHERE s.group_id=$1 AND s.status='confirmed'
		UNION ALL SELECT e.payer_user_id,a.currency,CASE WHEN a.type='refund' THEN -a.amount_minor ELSE a.amount_minor END FROM adjustments a JOIN expenses e ON e.id=a.expense_id WHERE a.group_id=$1
		UNION ALL SELECT aa.user_id,a.currency,CASE WHEN a.type='refund' THEN aa.amount_minor ELSE -aa.amount_minor END FROM adjustment_allocations aa JOIN adjustments a ON a.id=aa.adjustment_id WHERE a.group_id=$1
	) SELECT user_id,currency,SUM(amount)::bigint FROM entries GROUP BY user_id,currency ORDER BY currency,user_id`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var balances []domain.Balance
	for rows.Next() {
		var balance domain.Balance
		if err := rows.Scan(&balance.UserID, &balance.Currency, &balance.NetAmountMinor); err != nil {
			return nil, err
		}
		balances = append(balances, balance)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return balances, nil
}

func (s *Store) GetBalanceBreakdown(ctx context.Context, actorID, groupID, userID int64) ([]domain.BalanceEntry, []domain.Balance, error) {
	if err := s.ensureMember(ctx, actorID, groupID); err != nil {
		return nil, nil, err
	}
	rows, err := s.pool.Query(ctx, `SELECT operation_type,operation_id,currency,amount,occurred_at FROM (
		SELECT 'expense' operation_type,e.id operation_id,e.currency,e.amount_minor amount,e.created_at occurred_at FROM expenses e WHERE e.group_id=$1 AND e.status='confirmed' AND e.payer_user_id=$2
		UNION ALL SELECT 'allocation',e.id,e.currency,-a.amount_minor,e.created_at FROM allocations a JOIN expenses e ON e.id=a.expense_id WHERE e.group_id=$1 AND e.status='confirmed' AND a.user_id=$2
		UNION ALL SELECT 'settlement_sent',s.id,s.currency,s.amount_minor,s.created_at FROM settlements s WHERE s.group_id=$1 AND s.status='confirmed' AND s.sender_user_id=$2
		UNION ALL SELECT 'settlement_received',s.id,s.currency,-s.amount_minor,s.created_at FROM settlements s WHERE s.group_id=$1 AND s.status='confirmed' AND s.receiver_user_id=$2
		UNION ALL SELECT 'adjustment_payer',a.id,a.currency,CASE WHEN a.type='refund' THEN -a.amount_minor ELSE a.amount_minor END,a.created_at FROM adjustments a JOIN expenses e ON e.id=a.expense_id WHERE a.group_id=$1 AND e.payer_user_id=$2
		UNION ALL SELECT 'adjustment_allocation',a.id,a.currency,CASE WHEN a.type='refund' THEN aa.amount_minor ELSE -aa.amount_minor END,a.created_at FROM adjustment_allocations aa JOIN adjustments a ON a.id=aa.adjustment_id WHERE a.group_id=$1 AND aa.user_id=$2
	) operations ORDER BY occurred_at,operation_type,operation_id`, groupID, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var entries []domain.BalanceEntry
	for rows.Next() {
		var entry domain.BalanceEntry
		if err := rows.Scan(&entry.OperationType, &entry.OperationID, &entry.Currency, &entry.AmountMinor, &entry.OccurredAt); err != nil {
			return nil, nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	all, err := s.GetBalance(ctx, actorID, groupID)
	if err != nil {
		return nil, nil, err
	}
	balances := make([]domain.Balance, 0)
	for _, balance := range all {
		if balance.UserID == userID {
			balances = append(balances, balance)
		}
	}
	return entries, balances, nil
}
