package http

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"delim/internal/gateway/launch"
	"delim/internal/gateway/notifications"
	corev1 "delim/pkg/gen/core/v1"
	"github.com/go-chi/chi/v5"
)

type settlementClient interface {
	ledgerClient
	CreateSettlement(context.Context, *corev1.CreateSettlementRequest) (*corev1.CreateSettlementResponse, error)
	ConfirmSettlement(context.Context, *corev1.ConfirmSettlementRequest) (*corev1.ConfirmSettlementResponse, error)
	ListSettlements(context.Context, *corev1.ListSettlementsRequest) (*corev1.ListSettlementsResponse, error)
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

type settlementListResponse struct {
	Settlements []settlementResponse `json:"settlements"`
	NextCursor  int64                `json:"next_cursor,omitempty"`
}

func createSettlement(core settlementClient, notifier *notifications.Notifier) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		groupID, err := parseID(chi.URLParam(r, "groupID"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		if _, err := core.GetGroup(r.Context(), &corev1.GetGroupRequest{ActorUserId: actorID, GroupId: groupID}); err != nil {
			writeDownstreamError(w, err)
			return
		}
		var request createSettlementRequest
		if err := decodeJSON(w, r, &request); err != nil {
			writeError(w, http.StatusBadRequest, "malformed_request", "malformed request")
			return
		}
		response, err := core.CreateSettlement(r.Context(), &corev1.CreateSettlementRequest{
			ActorUserId: actorID, GroupId: groupID, SenderUserId: request.SenderUserID,
			ReceiverUserId: request.ReceiverUserID, AmountMinor: request.AmountMinor, Currency: normalizeCurrency(request.Currency),
		})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		settlement := response.GetSettlement()
		if notifier != nil && settlement != nil {
			text := fmt.Sprintf("Отмечено погашение %s. Получателю нужно подтвердить.", formatMoneyMinor(settlement.GetAmountMinor(), settlement.GetCurrency()))
			notifier.NotifyGroup(r.Context(), groupID, "settlement_created",
				notifications.SettlementKey(settlement.GetId()),
				notifications.Payload{Text: text, Buttons: []notifications.ButtonSpec{{Text: "Открыть", Action: launch.ActionSettlement, GroupID: groupID, Entity: settlement.GetId()}}})
		}
		writeJSON(w, http.StatusCreated, settlementToResponse(response.GetSettlement()))
	}
}

func confirmSettlement(core settlementClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		settlementID, err := parseID(chi.URLParam(r, "settlementID"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil {
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

func listSettlements(core settlementClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		groupID, err := parseID(chi.URLParam(r, "groupID"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group id")
			return
		}
		limit, err := parseLimit(r.URL.Query().Get("limit"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid limit")
			return
		}
		cursor, err := parseCursor(r.URL.Query().Get("cursor"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid cursor")
			return
		}
		response, err := core.ListSettlements(r.Context(), &corev1.ListSettlementsRequest{ActorUserId: actorID, GroupId: groupID, Page: &corev1.PageRequest{Limit: limit, CursorId: cursor}})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		settlements := make([]settlementResponse, 0, len(response.GetSettlements()))
		for _, settlement := range response.GetSettlements() {
			settlements = append(settlements, settlementToResponse(settlement))
		}
		writeJSON(w, http.StatusOK, settlementListResponse{Settlements: settlements, NextCursor: response.GetPage().GetNextCursorId()})
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
