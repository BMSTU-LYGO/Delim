package http

import (
	"context"
	"net/http"
	"strconv"
	"time"

	corev1 "delim/pkg/gen/core/v1"
	"github.com/go-chi/chi/v5"
)

type settlementClient interface {
	ledgerClient
	CreateSettlement(context.Context, *corev1.CreateSettlementRequest) (*corev1.CreateSettlementResponse, error)
	ConfirmSettlement(context.Context, *corev1.ConfirmSettlementRequest) (*corev1.ConfirmSettlementResponse, error)
}

type createSettlementRequest struct {
	SenderUserID   int64  `json:"sender_user_id"`
	ReceiverUserID int64  `json:"receiver_user_id"`
	AmountMinor    int64  `json:"amount_minor"`
	Currency       string `json:"currency"`
}

type settlementResponse struct {
	ID             int64      `json:"id"`
	GroupID        int64      `json:"group_id"`
	SenderUserID   int64      `json:"sender_user_id"`
	ReceiverUserID int64      `json:"receiver_user_id"`
	AmountMinor    int64      `json:"amount_minor"`
	Currency       string     `json:"currency"`
	Status         string     `json:"status"`
	CreatedBy      int64      `json:"created_by"`
	Version        int64      `json:"version"`
	CreatedAt      time.Time  `json:"created_at"`
	ConfirmedAt    *time.Time `json:"confirmed_at,omitempty"`
}

func createSettlement(core settlementClient) http.HandlerFunc {
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
		var request createSettlementRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		response, err := core.CreateSettlement(r.Context(), &corev1.CreateSettlementRequest{
			ActorUserId: actorID, GroupId: groupID, SenderUserId: request.SenderUserID,
			ReceiverUserId: request.ReceiverUserID, AmountMinor: request.AmountMinor, Currency: request.Currency,
		})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, settlementToResponse(response.GetSettlement()))
	}
}

func confirmSettlement(core settlementClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		settlementID, err := strconv.ParseInt(chi.URLParam(r, "settlementID"), 10, 64)
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil || settlementID <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid settlement id")
			return
		}
		response, err := core.ConfirmSettlement(r.Context(), &corev1.ConfirmSettlementRequest{ActorUserId: actorID, SettlementId: settlementID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, settlementToResponse(response.GetSettlement()))
	}
}

func settlementToResponse(settlement *corev1.Settlement) settlementResponse {
	if settlement == nil {
		return settlementResponse{}
	}
	response := settlementResponse{
		ID: settlement.Id, GroupID: settlement.GroupId, SenderUserID: settlement.SenderUserId,
		ReceiverUserID: settlement.ReceiverUserId, AmountMinor: settlement.AmountMinor, Currency: settlement.Currency,
		Status: settlementStatusName(settlement.Status), CreatedBy: settlement.CreatedBy, Version: settlement.Version,
		CreatedAt: settlement.CreatedAt.AsTime(),
	}
	if settlement.ConfirmedAt != nil {
		confirmedAt := settlement.ConfirmedAt.AsTime()
		response.ConfirmedAt = &confirmedAt
	}
	return response
}

func settlementStatusName(value corev1.SettlementStatus) string {
	switch value {
	case corev1.SettlementStatus_SETTLEMENT_STATUS_PENDING:
		return "pending"
	case corev1.SettlementStatus_SETTLEMENT_STATUS_CONFIRMED:
		return "confirmed"
	case corev1.SettlementStatus_SETTLEMENT_STATUS_CANCELLED:
		return "cancelled"
	default:
		return "unspecified"
	}
}
