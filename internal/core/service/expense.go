package service

import (
	"context"
	"delim/internal/core/domain"
	corev1 "delim/pkg/gen/core/v1"
	"time"
)

type ExpenseService interface {
	Create(context.Context, int64, domain.ExpenseInput) (domain.Expense, error)
	Get(context.Context, int64, int64) (domain.Expense, error)
	List(context.Context, int64, int64, int64, int32) ([]domain.Expense, error)
	Update(context.Context, int64, int64, int64, domain.ExpenseInput) (domain.Expense, error)
}

func (s *GRPCServer) UpdateExpense(ctx context.Context, req *corev1.UpdateExpenseRequest) (*corev1.UpdateExpenseResponse, error) {
	expense, err := s.expenses.Update(ctx, req.GetActorUserId(), req.GetExpenseId(), req.GetVersion(), expenseInputFromProto(req.GetExpense()))
	if err != nil {
		return nil, err
	}
	return &corev1.UpdateExpenseResponse{Expense: expenseToProto(expense)}, nil
}

func (s *GRPCServer) GetExpense(ctx context.Context, req *corev1.GetExpenseRequest) (*corev1.GetExpenseResponse, error) {
	expense, err := s.expenses.Get(ctx, req.GetActorUserId(), req.GetExpenseId())
	if err != nil {
		return nil, err
	}
	return &corev1.GetExpenseResponse{Expense: expenseToProto(expense)}, nil
}

func (s *GRPCServer) ListExpenses(ctx context.Context, req *corev1.ListExpensesRequest) (*corev1.ListExpensesResponse, error) {
	var cursor int64
	var limit int32
	if req.GetPage() != nil {
		cursor, limit = req.GetPage().GetCursorId(), req.GetPage().GetLimit()
	}
	expenses, err := s.expenses.List(ctx, req.GetActorUserId(), req.GetGroupId(), cursor, limit)
	if err != nil {
		return nil, err
	}
	response := &corev1.ListExpensesResponse{Expenses: make([]*corev1.Expense, 0, len(expenses)), Page: &corev1.PageResponse{}}
	for _, expense := range expenses {
		response.Expenses = append(response.Expenses, expenseToProto(expense))
	}
	if len(expenses) > 0 {
		response.Page.NextCursorId = expenses[len(expenses)-1].ID
	}
	return response, nil
}

func (s *GRPCServer) CreateExpense(ctx context.Context, req *corev1.CreateExpenseRequest) (*corev1.CreateExpenseResponse, error) {
	input := expenseInputFromProto(req.GetExpense())
	expense, err := s.expenses.Create(ctx, req.GetActorUserId(), input)
	if err != nil {
		return nil, err
	}
	return &corev1.CreateExpenseResponse{Expense: expenseToProto(expense)}, nil
}
func expenseInputFromProto(input *corev1.ExpenseInput) domain.ExpenseInput {
	if input == nil {
		return domain.ExpenseInput{}
	}
	result := domain.ExpenseInput{GroupID: input.GetGroupId(), PayerUserID: input.GetPayerUserId(), AmountMinor: input.GetAmountMinor(), Currency: input.GetCurrency(), Description: input.GetDescription(), ExpenseDate: time.Unix(input.GetExpenseDateUnix(), 0), SplitType: splitTypeFromProto(input.GetSplitType())}
	for _, p := range input.GetParticipants() {
		result.Participants = append(result.Participants, domain.SplitParticipant{UserID: p.GetUserId(), Value: p.GetValue()})
	}
	for _, item := range input.GetItems() {
		result.Items = append(result.Items, domain.ExpenseItemInput{Name: item.GetName(), AmountMinor: item.GetAmountMinor(), ParticipantUserIDs: append([]int64(nil), item.GetParticipantUserIds()...)})
	}
	return result
}
func expenseToProto(expense domain.Expense) *corev1.Expense {
	result := &corev1.Expense{Id: expense.ID, GroupId: expense.GroupID, PayerUserId: expense.PayerUserID, CreatedBy: expense.CreatedBy, AmountMinor: expense.AmountMinor, Currency: expense.Currency, Description: expense.Description, ExpenseDateUnix: expense.ExpenseDate.Unix(), SplitType: splitTypeToProto(expense.SplitType), Status: expenseStatusToProto(expense.Status), Version: expense.Version, CreatedAtUnix: expense.CreatedAt.Unix(), UpdatedAtUnix: expense.UpdatedAt.Unix()}
	for _, item := range expense.Items {
		result.Items = append(result.Items, &corev1.ExpenseItem{Id: item.ID, ExpenseId: item.ExpenseID, Name: item.Name, AmountMinor: item.AmountMinor, Position: item.Position})
	}
	for _, a := range expense.Allocations {
		var itemID int64
		if a.ExpenseItemID != nil {
			itemID = *a.ExpenseItemID
		}
		result.Allocations = append(result.Allocations, &corev1.Allocation{Id: a.ID, ExpenseId: a.ExpenseID, ExpenseItemId: itemID, UserId: a.UserID, AmountMinor: a.AmountMinor})
	}
	return result
}
func splitTypeFromProto(value corev1.SplitType) domain.SplitType {
	switch value {
	case corev1.SplitType_SPLIT_TYPE_EQUAL:
		return domain.SplitEqual
	case corev1.SplitType_SPLIT_TYPE_FIXED:
		return domain.SplitFixed
	case corev1.SplitType_SPLIT_TYPE_SHARES:
		return domain.SplitShares
	case corev1.SplitType_SPLIT_TYPE_PERCENTAGE:
		return domain.SplitPercentage
	case corev1.SplitType_SPLIT_TYPE_ITEM:
		return domain.SplitItem
	default:
		return ""
	}
}
func splitTypeToProto(value domain.SplitType) corev1.SplitType {
	switch value {
	case domain.SplitEqual:
		return corev1.SplitType_SPLIT_TYPE_EQUAL
	case domain.SplitFixed:
		return corev1.SplitType_SPLIT_TYPE_FIXED
	case domain.SplitShares:
		return corev1.SplitType_SPLIT_TYPE_SHARES
	case domain.SplitPercentage:
		return corev1.SplitType_SPLIT_TYPE_PERCENTAGE
	case domain.SplitItem:
		return corev1.SplitType_SPLIT_TYPE_ITEM
	default:
		return corev1.SplitType_SPLIT_TYPE_UNSPECIFIED
	}
}
func expenseStatusToProto(value domain.ExpenseStatus) corev1.ExpenseStatus {
	switch value {
	case domain.ExpensePending:
		return corev1.ExpenseStatus_EXPENSE_STATUS_PENDING
	case domain.ExpenseConfirmed:
		return corev1.ExpenseStatus_EXPENSE_STATUS_CONFIRMED
	case domain.ExpenseCancelled:
		return corev1.ExpenseStatus_EXPENSE_STATUS_CANCELLED
	default:
		return corev1.ExpenseStatus_EXPENSE_STATUS_UNSPECIFIED
	}
}
