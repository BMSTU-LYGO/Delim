package postgres

import (
	"context"
	"delim/internal/core/domain"
	"errors"
	"github.com/jackc/pgx/v5"
)

func scanExpense(row pgx.Row, expense *domain.Expense) error {
	return row.Scan(&expense.ID, &expense.GroupID, &expense.PayerUserID, &expense.CreatedBy, &expense.AmountMinor, &expense.Currency, &expense.Description, &expense.ExpenseDate, &expense.SplitType, &expense.Status, &expense.Version, &expense.CreatedAt, &expense.UpdatedAt)
}

func (s *Store) GetExpense(ctx context.Context, actorID, expenseID int64) (domain.Expense, error) {
	var expense domain.Expense
	err := scanExpense(s.pool.QueryRow(ctx, `SELECT e.id,e.group_id,e.payer_user_id,e.created_by,e.amount_minor,e.currency,e.description,e.expense_date,e.split_type,e.status,e.version,e.created_at,e.updated_at FROM expenses e JOIN group_members gm ON gm.group_id=e.group_id AND gm.user_id=$1 WHERE e.id=$2`, actorID, expenseID), &expense)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Expense{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Expense{}, err
	}
	expenses := []domain.Expense{expense}
	if err := s.loadExpenseDetails(ctx, expenses); err != nil {
		return domain.Expense{}, err
	}
	return expenses[0], nil
}

func (s *Store) ListExpenses(ctx context.Context, actorID, groupID, cursor int64, limit int32) ([]domain.Expense, error) {
	rows, err := s.pool.Query(ctx, `SELECT e.id,e.group_id,e.payer_user_id,e.created_by,e.amount_minor,e.currency,e.description,e.expense_date,e.split_type,e.status,e.version,e.created_at,e.updated_at FROM expenses e JOIN group_members gm ON gm.group_id=e.group_id AND gm.user_id=$1 WHERE e.group_id=$2 AND ($3=0 OR e.id<$3) ORDER BY e.id DESC LIMIT $4`, actorID, groupID, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	expenses := make([]domain.Expense, 0, limit)
	for rows.Next() {
		var expense domain.Expense
		if err := rows.Scan(&expense.ID, &expense.GroupID, &expense.PayerUserID, &expense.CreatedBy, &expense.AmountMinor, &expense.Currency, &expense.Description, &expense.ExpenseDate, &expense.SplitType, &expense.Status, &expense.Version, &expense.CreatedAt, &expense.UpdatedAt); err != nil {
			return nil, err
		}
		expenses = append(expenses, expense)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(expenses) == 0 {
		return expenses, nil
	}
	if err := s.loadExpenseDetails(ctx, expenses); err != nil {
		return nil, err
	}
	return expenses, nil
}

func (s *Store) loadExpenseDetails(ctx context.Context, expenses []domain.Expense) error {
	ids := make([]int64, len(expenses))
	byID := make(map[int64]*domain.Expense, len(expenses))
	for i := range expenses {
		ids[i] = expenses[i].ID
		byID[expenses[i].ID] = &expenses[i]
	}
	itemRows, err := s.pool.Query(ctx, `SELECT id,expense_id,name,amount_minor,position FROM expense_items WHERE expense_id=ANY($1) ORDER BY expense_id,position`, ids)
	if err != nil {
		return err
	}
	for itemRows.Next() {
		var item domain.ExpenseItem
		if err := itemRows.Scan(&item.ID, &item.ExpenseID, &item.Name, &item.AmountMinor, &item.Position); err != nil {
			itemRows.Close()
			return err
		}
		byID[item.ExpenseID].Items = append(byID[item.ExpenseID].Items, item)
	}
	if err := itemRows.Err(); err != nil {
		itemRows.Close()
		return err
	}
	itemRows.Close()
	allocationRows, err := s.pool.Query(ctx, `SELECT id,expense_id,expense_item_id,user_id,amount_minor FROM allocations WHERE expense_id=ANY($1) ORDER BY expense_id,id`, ids)
	if err != nil {
		return err
	}
	defer allocationRows.Close()
	for allocationRows.Next() {
		var allocation domain.Allocation
		if err := allocationRows.Scan(&allocation.ID, &allocation.ExpenseID, &allocation.ExpenseItemID, &allocation.UserID, &allocation.AmountMinor); err != nil {
			return err
		}
		byID[allocation.ExpenseID].Allocations = append(byID[allocation.ExpenseID].Allocations, allocation)
	}
	return allocationRows.Err()
}

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

func (s *Store) UpdateExpense(ctx context.Context, actorID, expenseID, version int64, input domain.ExpenseInput, drafts []domain.AllocationDraft) (domain.Expense, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Expense{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var expense domain.Expense
	err = scanExpense(tx.QueryRow(ctx, `UPDATE expenses SET payer_user_id=$1,amount_minor=$2,currency=$3,description=$4,expense_date=$5,split_type=$6,version=version+1,updated_at=NOW() WHERE id=$7 AND version=$8 AND status='pending' RETURNING id,group_id,payer_user_id,created_by,amount_minor,currency,description,expense_date,split_type,status,version,created_at,updated_at`, input.PayerUserID, input.AmountMinor, input.Currency, input.Description, input.ExpenseDate, input.SplitType, expenseID, version), &expense)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Expense{}, domain.ErrConflict
	}
	if err != nil {
		return domain.Expense{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM allocations WHERE expense_id=$1`, expenseID); err != nil {
		return domain.Expense{}, err
	}
	if _, err = tx.Exec(ctx, `DELETE FROM expense_items WHERE expense_id=$1`, expenseID); err != nil {
		return domain.Expense{}, err
	}
	for position, item := range input.Items {
		var saved domain.ExpenseItem
		err = tx.QueryRow(ctx, `INSERT INTO expense_items(expense_id,name,amount_minor,position) VALUES($1,$2,$3,$4) RETURNING id,expense_id,name,amount_minor,position`, expenseID, item.Name, item.AmountMinor, position).Scan(&saved.ID, &saved.ExpenseID, &saved.Name, &saved.AmountMinor, &saved.Position)
		if err != nil {
			return domain.Expense{}, err
		}
		expense.Items = append(expense.Items, saved)
	}
	for _, draft := range drafts {
		var itemID *int64
		if draft.ItemIndex != nil {
			id := expense.Items[*draft.ItemIndex].ID
			itemID = &id
		}
		var saved domain.Allocation
		err = tx.QueryRow(ctx, `INSERT INTO allocations(expense_id,expense_item_id,user_id,amount_minor) VALUES($1,$2,$3,$4) RETURNING id,expense_id,expense_item_id,user_id,amount_minor`, expenseID, itemID, draft.UserID, draft.AmountMinor).Scan(&saved.ID, &saved.ExpenseID, &saved.ExpenseItemID, &saved.UserID, &saved.AmountMinor)
		if err != nil {
			return domain.Expense{}, err
		}
		expense.Allocations = append(expense.Allocations, saved)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(group_id,actor_user_id,action,entity_type,entity_id,entity_version,metadata) VALUES($1,$2,'expense.updated','expense',$3,$4,'{}')`, expense.GroupID, actorID, expense.ID, expense.Version); err != nil {
		return domain.Expense{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Expense{}, err
	}
	return expense, nil
}

func (s *Store) ConfirmExpense(ctx context.Context, actorID, expenseID int64) (domain.Expense, error) {
	return s.changeExpenseStatus(ctx, actorID, expenseID, domain.ExpenseConfirmed)
}

func (s *Store) changeExpenseStatus(ctx context.Context, actorID, expenseID int64, target domain.ExpenseStatus) (domain.Expense, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Expense{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var expense domain.Expense
	err = scanExpense(tx.QueryRow(ctx, `SELECT e.id,e.group_id,e.payer_user_id,e.created_by,e.amount_minor,e.currency,e.description,e.expense_date,e.split_type,e.status,e.version,e.created_at,e.updated_at FROM expenses e JOIN group_members gm ON gm.group_id=e.group_id AND gm.user_id=$1 WHERE e.id=$2 FOR UPDATE OF e`, actorID, expenseID), &expense)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Expense{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Expense{}, err
	}
	if expense.Status == target {
		if err = tx.Commit(ctx); err != nil {
			return domain.Expense{}, err
		}
		return expense, nil
	}
	if expense.Status != domain.ExpensePending {
		return domain.Expense{}, domain.ErrInvalidState
	}
	err = tx.QueryRow(ctx, `UPDATE expenses SET status=$1,version=version+1,updated_at=NOW() WHERE id=$2 RETURNING status,version,updated_at`, target, expenseID).Scan(&expense.Status, &expense.Version, &expense.UpdatedAt)
	if err != nil {
		return domain.Expense{}, err
	}
	action := "expense.confirmed"
	if target == domain.ExpenseCancelled {
		action = "expense.cancelled"
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(group_id,actor_user_id,action,entity_type,entity_id,entity_version,metadata) VALUES($1,$2,$3,'expense',$4,$5,'{}')`, expense.GroupID, actorID, action, expense.ID, expense.Version); err != nil {
		return domain.Expense{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Expense{}, err
	}
	return expense, nil
}
