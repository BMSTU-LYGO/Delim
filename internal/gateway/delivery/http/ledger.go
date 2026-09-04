package http

import (
	"context"
	"net/http"
	"time"

	corev1 "delim/pkg/gen/core/v1"
	"github.com/go-chi/chi/v5"
)

type ledgerClient interface {
	expenseClient
	GetBalance(context.Context, *corev1.GetBalanceRequest) (*corev1.GetBalanceResponse, error)
	GetBalanceBreakdown(context.Context, *corev1.GetBalanceBreakdownRequest) (*corev1.GetBalanceBreakdownResponse, error)
	GetSettlementPlan(context.Context, *corev1.GetSettlementPlanRequest) (*corev1.GetSettlementPlanResponse, error)
}

type balanceResponse struct {
	UserID         int64  `json:"user_id"`
	Currency       string `json:"currency"`
	NetAmountMinor int64  `json:"net_amount_minor"`
}

type balanceEntryResponse struct {
	OperationType string    `json:"operation_type"`
	OperationID   int64     `json:"operation_id"`
	Currency      string    `json:"currency"`
	AmountMinor   int64     `json:"amount_minor"`
	OccurredAt    time.Time `json:"occurred_at"`
}

type balanceBreakdownResponse struct {
	Balance []balanceResponse      `json:"balance"`
	Entries []balanceEntryResponse `json:"entries"`
}

type settlementPlanTransferResponse struct {
	FromUserID  int64  `json:"from_user_id"`
	ToUserID    int64  `json:"to_user_id"`
	AmountMinor int64  `json:"amount_minor"`
	Currency    string `json:"currency"`
}

func getBalance(core ledgerClient) http.HandlerFunc {
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
		response, err := core.GetBalance(r.Context(), &corev1.GetBalanceRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		balances := make([]balanceResponse, 0, len(response.GetBalances()))
		for _, balance := range response.GetBalances() {
			balances = append(balances, balanceToResponse(balance))
		}
		writeJSON(w, http.StatusOK, balances)
	}
}

func getBalanceBreakdown(core ledgerClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		actorID, ok := userIDFromContext(r.Context())
		groupID, groupErr := parseID(chi.URLParam(r, "groupID"))
		userID, userErr := parseID(chi.URLParam(r, "userID"))
		if !ok {
			writeError(w, http.StatusUnauthorized, "invalid_session", "invalid or expired session")
			return
		}
		if groupErr != nil || userErr != nil {
			writeError(w, http.StatusBadRequest, "invalid_argument", "invalid group or user id")
			return
		}
		response, err := core.GetBalanceBreakdown(r.Context(), &corev1.GetBalanceBreakdownRequest{ActorUserId: actorID, GroupId: groupID, UserId: userID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		balances := make([]balanceResponse, 0, len(response.GetBalances()))
		for _, balance := range response.GetBalances() {
			balances = append(balances, balanceToResponse(balance))
		}
		entries := make([]balanceEntryResponse, 0, len(response.GetEntries()))
		for _, entry := range response.GetEntries() {
			entries = append(entries, balanceEntryResponse{
				OperationType: entry.OperationType, OperationID: entry.OperationId, Currency: entry.Currency,
				AmountMinor: entry.AmountMinor, OccurredAt: entry.OccurredAt.AsTime(),
			})
		}
		writeJSON(w, http.StatusOK, balanceBreakdownResponse{Balance: balances, Entries: entries})
	}
}

func getSettlementPlan(core ledgerClient) http.HandlerFunc {
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
		response, err := core.GetSettlementPlan(r.Context(), &corev1.GetSettlementPlanRequest{ActorUserId: actorID, GroupId: groupID})
		if err != nil {
			writeDownstreamError(w, err)
			return
		}
		transfers := make([]settlementPlanTransferResponse, 0, len(response.GetTransfers()))
		for _, transfer := range response.GetTransfers() {
			transfers = append(transfers, settlementPlanTransferResponse{
				FromUserID: transfer.FromUserId, ToUserID: transfer.ToUserId,
				AmountMinor: transfer.AmountMinor, Currency: transfer.Currency,
			})
		}
		writeJSON(w, http.StatusOK, transfers)
	}
}

func balanceToResponse(balance *corev1.Balance) balanceResponse {
	if balance == nil {
		return balanceResponse{}
	}
	return balanceResponse{UserID: balance.UserId, Currency: balance.Currency, NetAmountMinor: balance.NetAmountMinor}
}
