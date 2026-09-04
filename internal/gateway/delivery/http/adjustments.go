package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	corev1 "delim/pkg/gen/core/v1"
	"github.com/go-chi/chi/v5"
)

type adjustmentClient interface {
	settlementClient
	CreateAdjustment(context.Context, *corev1.CreateAdjustmentRequest) (*corev1.CreateAdjustmentResponse, error)
}

type createAdjustmentRequest struct {
	Type        string                        `json:"type"`
	AmountMinor int64                         `json:"amount_minor"`
	Currency    string                        `json:"currency"`
	Allocations []adjustmentAllocationRequest `json:"allocations"`
}

type adjustmentAllocationRequest struct {
	UserID      int64 `json:"user_id"`
	AmountMinor int64 `json:"amount_minor"`
}

type adjustmentResponse struct {
	ID          int64                          `json:"id"`
	GroupID     int64                          `json:"group_id"`
	ExpenseID   int64                          `json:"expense_id"`
	Type        string                         `json:"type"`
	AmountMinor int64                          `json:"amount_minor"`
	Currency    string                         `json:"currency"`
	CreatedBy   int64                          `json:"created_by"`
	CreatedAt   time.Time                      `json:"created_at"`
	Allocations []adjustmentAllocationResponse `json:"allocations"`
}

type adjustmentAllocationResponse struct {
	UserID      int64 `json:"user_id"`
	AmountMinor int64 `json:"amount_minor"`
}

func createAdjustment(core adjustmentClient) http.HandlerFunc {
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
		var request createAdjustmentRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		adjustmentType, valid := adjustmentTypeFromName(request.Type)
		if !valid {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid adjustment type")
			return
		}
		allocations := make([]*corev1.AdjustmentAllocation, 0, len(request.Allocations))
		for _, allocation := range request.Allocations {
			allocations = append(allocations, &corev1.AdjustmentAllocation{UserId: allocation.UserID, AmountMinor: allocation.AmountMinor})
		}
		response, err := core.CreateAdjustment(r.Context(), &corev1.CreateAdjustmentRequest{
			ActorUserId: actorID, ExpenseId: expenseID, Type: adjustmentType,
			AmountMinor: request.AmountMinor, Currency: request.Currency, Allocations: allocations,
		})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, adjustmentToResponse(response.GetAdjustment()))
	}
}

func adjustmentTypeFromName(value string) (corev1.AdjustmentType, bool) {
	switch value {
	case "refund":
		return corev1.AdjustmentType_ADJUSTMENT_TYPE_REFUND, true
	case "correction":
		return corev1.AdjustmentType_ADJUSTMENT_TYPE_CORRECTION, true
	default:
		return corev1.AdjustmentType_ADJUSTMENT_TYPE_UNSPECIFIED, false
	}
}

func adjustmentTypeName(value corev1.AdjustmentType) string {
	switch value {
	case corev1.AdjustmentType_ADJUSTMENT_TYPE_REFUND:
		return "refund"
	case corev1.AdjustmentType_ADJUSTMENT_TYPE_CORRECTION:
		return "correction"
	default:
		return "unspecified"
	}
}

func adjustmentToResponse(adjustment *corev1.Adjustment) adjustmentResponse {
	if adjustment == nil {
		return adjustmentResponse{}
	}
	allocations := make([]adjustmentAllocationResponse, 0, len(adjustment.GetAllocations()))
	for _, allocation := range adjustment.GetAllocations() {
		allocations = append(allocations, adjustmentAllocationResponse{UserID: allocation.UserId, AmountMinor: allocation.AmountMinor})
	}
	return adjustmentResponse{
		ID: adjustment.Id, GroupID: adjustment.GroupId, ExpenseID: adjustment.ExpenseId,
		Type: adjustmentTypeName(adjustment.Type), AmountMinor: adjustment.AmountMinor, Currency: adjustment.Currency,
		CreatedBy: adjustment.CreatedBy, CreatedAt: adjustment.CreatedAt.AsTime(), Allocations: allocations,
	}
}
