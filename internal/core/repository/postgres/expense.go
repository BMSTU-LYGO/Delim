package postgres

import (
	"context"
	"delim/internal/core/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) GroupStateAndMembers(ctx context.Context, groupID int64) (domain.GroupStatus, []int64, error) {
	rows, err := s.pool.Query(ctx, `SELECT g.status,gm.user_id FROM groups g JOIN group_members gm ON gm.group_id=g.id WHERE g.id=$1 ORDER BY gm.user_id`, groupID)
	if err != nil {
		return "", nil, err
	}
	defer rows.Close()
	var status domain.GroupStatus
	var members []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&status, &id); err != nil {
			return "", nil, err
		}
		members = append(members, id)
	}
	if err := rows.Err(); err != nil {
		return "", nil, err
	}
	if len(members) == 0 {
		return "", nil, domain.ErrNotFound
	}
	return status, members, nil
}

func (s *Store) CreateExpense(ctx context.Context, actorID int64, input domain.ExpenseInput, drafts []domain.AllocationDraft) (domain.Expense, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Expense{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var expense domain.Expense
	err = tx.QueryRow(ctx, `INSERT INTO expenses(group_id,payer_user_id,created_by,amount_minor,currency,description,expense_date,split_type) VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id,group_id,payer_user_id,created_by,amount_minor,currency,description,expense_date,split_type,status,version,created_at,updated_at`, input.GroupID, input.PayerUserID, actorID, input.AmountMinor, input.Currency, input.Description, input.ExpenseDate, input.SplitType).
		Scan(&expense.ID, &expense.GroupID, &expense.PayerUserID, &expense.CreatedBy, &expense.AmountMinor, &expense.Currency, &expense.Description, &expense.ExpenseDate, &expense.SplitType, &expense.Status, &expense.Version, &expense.CreatedAt, &expense.UpdatedAt)
	if err != nil {
		return domain.Expense{}, err
	}
	expense.Items = make([]domain.ExpenseItem, 0, len(input.Items))
	for position, item := range input.Items {
		var saved domain.ExpenseItem
		err = tx.QueryRow(ctx, `INSERT INTO expense_items(expense_id,name,amount_minor,position) VALUES($1,$2,$3,$4) RETURNING id,expense_id,name,amount_minor,position`, expense.ID, item.Name, item.AmountMinor, position).Scan(&saved.ID, &saved.ExpenseID, &saved.Name, &saved.AmountMinor, &saved.Position)
		if err != nil {
			return domain.Expense{}, err
		}
		expense.Items = append(expense.Items, saved)
	}
	expense.Allocations = make([]domain.Allocation, 0, len(drafts))
	for _, draft := range drafts {
		var itemID *int64
		if draft.ItemIndex != nil {
			id := expense.Items[*draft.ItemIndex].ID
			itemID = &id
		}
		var saved domain.Allocation
		err = tx.QueryRow(ctx, `INSERT INTO allocations(expense_id,expense_item_id,user_id,amount_minor) VALUES($1,$2,$3,$4) RETURNING id,expense_id,expense_item_id,user_id,amount_minor`, expense.ID, itemID, draft.UserID, draft.AmountMinor).Scan(&saved.ID, &saved.ExpenseID, &saved.ExpenseItemID, &saved.UserID, &saved.AmountMinor)
		if err != nil {
			return domain.Expense{}, err
		}
		expense.Allocations = append(expense.Allocations, saved)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(group_id,actor_user_id,action,entity_type,entity_id,entity_version,metadata) VALUES($1,$2,'expense.created','expense',$3,$4,'{}')`, expense.GroupID, actorID, expense.ID, expense.Version); err != nil {
		return domain.Expense{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Expense{}, err
	}
	return expense, nil
}
