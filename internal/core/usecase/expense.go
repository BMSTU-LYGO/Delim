package usecase

import (
	"context"
	"strings"
	"unicode"

	"delim/internal/core/domain"
	"delim/internal/core/domain/split"
)

type ExpenseRepository interface {
	GroupStateAndMembers(context.Context, int64) (domain.GroupStatus, []int64, error)
	CreateExpense(context.Context, int64, domain.ExpenseInput, []domain.AllocationDraft) (domain.Expense, error)
	GetExpense(context.Context, int64, int64) (domain.Expense, error)
	ListExpenses(context.Context, int64, int64, int64, int32) ([]domain.Expense, error)
	UpdateExpense(context.Context, int64, int64, int64, domain.ExpenseInput, []domain.AllocationDraft) (domain.Expense, error)
	ConfirmExpense(context.Context, int64, int64) (domain.Expense, error)
	CancelExpense(context.Context, int64, int64) (domain.Expense, error)
}

func (e *Expenses) Cancel(ctx context.Context, actorID, expenseID int64) (domain.Expense, error) {
	if actorID <= 0 || expenseID <= 0 {
		return domain.Expense{}, domain.ErrInvalidArgument
	}
	return e.repository.CancelExpense(ctx, actorID, expenseID)
}

type Expenses struct{ repository ExpenseRepository }

func NewExpenses(repository ExpenseRepository) *Expenses { return &Expenses{repository: repository} }

func (e *Expenses) Confirm(ctx context.Context, actorID, expenseID int64) (domain.Expense, error) {
	if actorID <= 0 || expenseID <= 0 {
		return domain.Expense{}, domain.ErrInvalidArgument
	}
	return e.repository.ConfirmExpense(ctx, actorID, expenseID)
}

func (e *Expenses) Get(ctx context.Context, actorID, expenseID int64) (domain.Expense, error) {
	if actorID <= 0 || expenseID <= 0 {
		return domain.Expense{}, domain.ErrInvalidArgument
	}
	return e.repository.GetExpense(ctx, actorID, expenseID)
}
func (e *Expenses) List(ctx context.Context, actorID, groupID, cursor int64, limit int32) ([]domain.Expense, error) {
	if actorID <= 0 || groupID <= 0 || cursor < 0 {
		return nil, domain.ErrInvalidArgument
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 0 || limit > 100 {
		return nil, domain.ErrInvalidArgument
	}
	return e.repository.ListExpenses(ctx, actorID, groupID, cursor, limit)
}

func (e *Expenses) Create(ctx context.Context, actorID int64, input domain.ExpenseInput) (domain.Expense, error) {
	if actorID <= 0 || validateExpenseInput(input) != nil {
		return domain.Expense{}, domain.ErrInvalidArgument
	}
	drafts, err := e.prepare(ctx, actorID, &input)
	if err != nil {
		return domain.Expense{}, err
	}
	return e.repository.CreateExpense(ctx, actorID, input, drafts)
}

func (e *Expenses) Update(ctx context.Context, actorID, expenseID, version int64, input domain.ExpenseInput) (domain.Expense, error) {
	if actorID <= 0 || expenseID <= 0 || version <= 0 || validateExpenseInput(input) != nil {
		return domain.Expense{}, domain.ErrInvalidArgument
	}
	current, err := e.repository.GetExpense(ctx, actorID, expenseID)
	if err != nil {
		return domain.Expense{}, err
	}
	if err := validateExpenseUpdate(current, version, input.GroupID); err != nil {
		return domain.Expense{}, err
	}
	drafts, err := e.prepare(ctx, actorID, &input)
	if err != nil {
		return domain.Expense{}, err
	}
	return e.repository.UpdateExpense(ctx, actorID, expenseID, version, input, drafts)
}

func validateExpenseInput(input domain.ExpenseInput) error {
	if input.GroupID <= 0 || input.PayerUserID <= 0 || input.AmountMinor <= 0 || !validCurrency(input.Currency) || input.ExpenseDate.IsZero() {
		return domain.ErrInvalidArgument
	}
	switch input.SplitType {
	case domain.SplitEqual, domain.SplitFixed, domain.SplitShares, domain.SplitPercentage:
		if len(input.Items) != 0 || len(input.Participants) == 0 {
			return domain.ErrInvalidArgument
		}
	case domain.SplitItem:
		if len(input.Participants) != 0 || len(input.Items) == 0 {
			return domain.ErrInvalidArgument
		}
	default:
		return domain.ErrInvalidArgument
	}
	seen := map[int64]struct{}{}
	for _, participant := range input.Participants {
		if participant.UserID <= 0 {
			return domain.ErrInvalidArgument
		}
		if _, ok := seen[participant.UserID]; ok {
			return domain.ErrInvalidArgument
		}
		seen[participant.UserID] = struct{}{}
	}
	for _, item := range input.Items {
		itemSeen := map[int64]struct{}{}
		if len(item.ParticipantUserIDs) == 0 {
			return domain.ErrInvalidArgument
		}
		for _, id := range item.ParticipantUserIDs {
			if id <= 0 {
				return domain.ErrInvalidArgument
			}
			if _, ok := itemSeen[id]; ok {
				return domain.ErrInvalidArgument
			}
			itemSeen[id] = struct{}{}
		}
	}
	return nil
}

func validateExpenseUpdate(current domain.Expense, version, groupID int64) error {
	if current.Status != domain.ExpensePending {
		return domain.ErrInvalidState
	}
	if current.Version != version {
		return domain.ErrConflict
	}
	if current.GroupID != groupID {
		return domain.ErrInvalidArgument
	}
	return nil
}

func (e *Expenses) prepare(ctx context.Context, actorID int64, input *domain.ExpenseInput) ([]domain.AllocationDraft, error) {
	status, memberIDs, err := e.repository.GroupStateAndMembers(ctx, input.GroupID)
	if err != nil {
		return nil, err
	}
	if status == domain.GroupArchived {
		return nil, domain.ErrArchivedGroup
	}
	members := make(map[int64]struct{}, len(memberIDs))
	for _, id := range memberIDs {
		members[id] = struct{}{}
	}
	if _, ok := members[actorID]; !ok {
		return nil, domain.ErrForbidden
	}
	if _, ok := members[input.PayerUserID]; !ok {
		return nil, domain.ErrInvalidArgument
	}
	input.Description = strings.TrimSpace(input.Description)
	var drafts []domain.AllocationDraft
	if input.SplitType == domain.SplitItem {
		items := make([]split.Item, len(input.Items))
		for i, item := range input.Items {
			if strings.TrimSpace(item.Name) == "" {
				return nil, domain.ErrInvalidArgument
			}
			for _, id := range item.ParticipantUserIDs {
				if _, ok := members[id]; !ok {
					return nil, domain.ErrInvalidArgument
				}
			}
			items[i] = split.Item{AmountMinor: item.AmountMinor, ParticipantIDs: item.ParticipantUserIDs}
			input.Items[i].Name = strings.TrimSpace(item.Name)
		}
		allocations, err := split.Items(input.AmountMinor, items)
		if err != nil {
			return nil, err
		}
		for _, allocation := range allocations {
			index := allocation.ItemIndex
			drafts = append(drafts, domain.AllocationDraft{ItemIndex: &index, UserID: allocation.UserID, AmountMinor: allocation.AmountMinor})
		}
	} else {
		values := make([]split.Allocation, len(input.Participants))
		ids := make([]int64, len(input.Participants))
		for i, p := range input.Participants {
			values[i] = split.Allocation{UserID: p.UserID, AmountMinor: p.Value}
			ids[i] = p.UserID
			if _, ok := members[p.UserID]; !ok {
				return nil, domain.ErrInvalidArgument
			}
		}
		var allocations []split.Allocation
		switch input.SplitType {
		case domain.SplitEqual:
			allocations, err = split.Equal(input.AmountMinor, ids)
		case domain.SplitFixed:
			allocations, err = split.Fixed(input.AmountMinor, values, memberIDs)
		case domain.SplitShares:
			allocations, err = split.Shares(input.AmountMinor, values, memberIDs)
		case domain.SplitPercentage:
			allocations, err = split.Percentage(input.AmountMinor, values, memberIDs)
		default:
			return nil, domain.ErrInvalidArgument
		}
		if err != nil {
			return nil, err
		}
		for _, a := range allocations {
			drafts = append(drafts, domain.AllocationDraft{UserID: a.UserID, AmountMinor: a.AmountMinor})
		}
	}
	return drafts, nil
}

func validCurrency(currency string) bool {
	if len(currency) != 3 {
		return false
	}
	for _, r := range currency {
		if !unicode.IsUpper(r) || r > 'Z' {
			return false
		}
	}
	return true
}
