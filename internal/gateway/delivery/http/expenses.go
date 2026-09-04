package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	corev1 "delim/pkg/gen/core/v1"
	"github.com/go-chi/chi/v5"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type expenseClient interface {
	groupClient
	CreateExpense(context.Context, *corev1.CreateExpenseRequest) (*corev1.CreateExpenseResponse, error)
	GetExpense(context.Context, *corev1.GetExpenseRequest) (*corev1.GetExpenseResponse, error)
	ListExpenses(context.Context, *corev1.ListExpensesRequest) (*corev1.ListExpensesResponse, error)
}

type expenseInputRequest struct {
	PayerUserID  int64                     `json:"payer_user_id"`
	AmountMinor  int64                     `json:"amount_minor"`
	Currency     string                    `json:"currency"`
	Description  string                    `json:"description"`
	ExpenseDate  string                    `json:"expense_date"`
	SplitType    string                    `json:"split_type"`
	Participants []splitParticipantRequest `json:"participants"`
	Items        []expenseItemRequest      `json:"items"`
}

type splitParticipantRequest struct {
	UserID int64 `json:"user_id"`
	Value  int64 `json:"value"`
}

type expenseItemRequest struct {
	Name               string  `json:"name"`
	AmountMinor        int64   `json:"amount_minor"`
	ParticipantUserIDs []int64 `json:"participant_user_ids"`
}

type expenseResponse struct {
	ID          int64                 `json:"id"`
	GroupID     int64                 `json:"group_id"`
	PayerUserID int64                 `json:"payer_user_id"`
	CreatedBy   int64                 `json:"created_by"`
	AmountMinor int64                 `json:"amount_minor"`
	Currency    string                `json:"currency"`
	Description string                `json:"description"`
	ExpenseDate time.Time             `json:"expense_date"`
	SplitType   string                `json:"split_type"`
	Status      string                `json:"status"`
	Version     int64                 `json:"version"`
	CreatedAt   time.Time             `json:"created_at"`
	UpdatedAt   time.Time             `json:"updated_at"`
	Items       []expenseItemResponse `json:"items"`
	Allocations []allocationResponse  `json:"allocations"`
}

type expenseItemResponse struct {
	ID          int64  `json:"id"`
	ExpenseID   int64  `json:"expense_id"`
	Name        string `json:"name"`
	AmountMinor int64  `json:"amount_minor"`
	Position    int32  `json:"position"`
}

type allocationResponse struct {
	ID            int64 `json:"id"`
	ExpenseID     int64 `json:"expense_id"`
	ExpenseItemID int64 `json:"expense_item_id,omitempty"`
	UserID        int64 `json:"user_id"`
	AmountMinor   int64 `json:"amount_minor"`
}

type expenseListResponse struct {
	Expenses   []expenseResponse `json:"expenses"`
	NextCursor int64             `json:"next_cursor,omitempty"`
}

func createExpense(core expenseClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil || groupID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		var request expenseInputRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		input, err := expenseInputToProto(groupID, request)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", err.Error())
			return
		}
		response, err := core.CreateExpense(r.Context(), &corev1.CreateExpenseRequest{ActorUserId: actorID, Expense: input})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, expenseToResponse(response.GetExpense()))
	}
}

func getExpense(core expenseClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		expenseID, err := strconv.ParseInt(chi.URLParam(r, "expenseID"), 10, 64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil || expenseID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid expense id")
			return
		}
		response, err := core.GetExpense(r.Context(), &corev1.GetExpenseRequest{ActorUserId: actorID, ExpenseId: expenseID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, expenseToResponse(response.GetExpense()))
	}
}

func listExpenses(core expenseClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		groupID, err := strconv.ParseInt(chi.URLParam(r, "groupID"), 10, 64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil || groupID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		limit := int64(50)
		if raw := r.URL.Query().Get("limit"); raw != "" {
			limit, err = strconv.ParseInt(raw, 10, 32)
			if err != nil || limit <= 0 || limit > 100 {
				writeError(w, http.StatusBadRequest, "invalid_argument", "invalid limit")
				return
			}
		}
		var cursor int64
		if raw := r.URL.Query().Get("cursor"); raw != "" {
			cursor, err = strconv.ParseInt(raw, 10, 64)
			if err != nil || cursor < 0 {
				writeError(w, http.StatusBadRequest, "invalid_argument", "invalid cursor")
				return
			}
		}
		response, err := core.ListExpenses(r.Context(), &corev1.ListExpensesRequest{ActorUserId: actorID, GroupId: groupID, Page: &corev1.PageRequest{Limit: int32(limit), CursorId: cursor}})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		expenses := make([]expenseResponse, 0, len(response.GetExpenses()))
		for _, expense := range response.GetExpenses() {
			expenses = append(expenses, expenseToResponse(expense))
		}
		writeJSON(w, http.StatusOK, expenseListResponse{Expenses: expenses, NextCursor: response.GetPage().GetNextCursorId()})
	}
}

func expenseInputToProto(groupID int64, request expenseInputRequest) (*corev1.ExpenseInput, error) {
	expenseDate, err := time.Parse(time.RFC3339, request.ExpenseDate)
	if err != nil {
		return nil, err
	}
	splitType, ok := splitTypeFromName(request.SplitType)
	if !ok {
		return nil, &validationError{message: "invalid split type"}
	}
	participants := make([]*corev1.SplitParticipant, 0, len(request.Participants))
	for _, participant := range request.Participants {
		participants = append(participants, &corev1.SplitParticipant{UserId: participant.UserID, Value: participant.Value})
	}
	items := make([]*corev1.ExpenseItemInput, 0, len(request.Items))
	for _, item := range request.Items {
		items = append(items, &corev1.ExpenseItemInput{Name: item.Name, AmountMinor: item.AmountMinor, ParticipantUserIds: item.ParticipantUserIDs})
	}
	return &corev1.ExpenseInput{
		GroupId: groupID, PayerUserId: request.PayerUserID, AmountMinor: request.AmountMinor,
		Currency: request.Currency, Description: request.Description, ExpenseDate: timestamppb.New(expenseDate),
		SplitType: splitType, Participants: participants, Items: items,
	}, nil
}

type validationError struct{ message string }

func (e *validationError) Error() string { return e.message }

func expenseToResponse(expense *corev1.Expense) expenseResponse {
	if expense == nil {
		return expenseResponse{}
	}
	items := make([]expenseItemResponse, 0, len(expense.GetItems()))
	for _, item := range expense.GetItems() {
		items = append(items, expenseItemResponse{ID: item.Id, ExpenseID: item.ExpenseId, Name: item.Name, AmountMinor: item.AmountMinor, Position: item.Position})
	}
	allocations := make([]allocationResponse, 0, len(expense.GetAllocations()))
	for _, allocation := range expense.GetAllocations() {
		allocations = append(allocations, allocationResponse{ID: allocation.Id, ExpenseID: allocation.ExpenseId, ExpenseItemID: allocation.ExpenseItemId, UserID: allocation.UserId, AmountMinor: allocation.AmountMinor})
	}
	return expenseResponse{
		ID: expense.Id, GroupID: expense.GroupId, PayerUserID: expense.PayerUserId, CreatedBy: expense.CreatedBy,
		AmountMinor: expense.AmountMinor, Currency: expense.Currency, Description: expense.Description,
		ExpenseDate: expense.ExpenseDate.AsTime(), SplitType: splitTypeName(expense.SplitType), Status: expenseStatusName(expense.Status), Version: expense.Version,
		CreatedAt: expense.CreatedAt.AsTime(), UpdatedAt: expense.UpdatedAt.AsTime(), Items: items, Allocations: allocations,
	}
}

func splitTypeFromName(value string) (corev1.SplitType, bool) {
	switch value {
	case "equal":
		return corev1.SplitType_SPLIT_TYPE_EQUAL, true
	case "fixed":
		return corev1.SplitType_SPLIT_TYPE_FIXED, true
	case "shares":
		return corev1.SplitType_SPLIT_TYPE_SHARES, true
	case "percentage":
		return corev1.SplitType_SPLIT_TYPE_PERCENTAGE, true
	case "item":
		return corev1.SplitType_SPLIT_TYPE_ITEM, true
	default:
		return corev1.SplitType_SPLIT_TYPE_UNSPECIFIED, false
	}
}

func splitTypeName(value corev1.SplitType) string {
	name, _ := map[corev1.SplitType]string{
		corev1.SplitType_SPLIT_TYPE_EQUAL: "equal", corev1.SplitType_SPLIT_TYPE_FIXED: "fixed",
		corev1.SplitType_SPLIT_TYPE_SHARES: "shares", corev1.SplitType_SPLIT_TYPE_PERCENTAGE: "percentage",
		corev1.SplitType_SPLIT_TYPE_ITEM: "item",
	}[value]
	if name == "" {
		return "unspecified"
	}
	return name
}

func expenseStatusName(value corev1.ExpenseStatus) string {
	switch value {
	case corev1.ExpenseStatus_EXPENSE_STATUS_PENDING:
		return "pending"
	case corev1.ExpenseStatus_EXPENSE_STATUS_CONFIRMED:
		return "confirmed"
	case corev1.ExpenseStatus_EXPENSE_STATUS_CANCELLED:
		return "cancelled"
	default:
		return "unspecified"
	}
}
