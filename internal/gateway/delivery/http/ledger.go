package http

import (
	"context"
	"net/http"
	"strconv"

	corev1 "delim/pkg/gen/core/v1"
	"github.com/go-chi/chi/v5"
)

type ledgerClient interface {
	expenseClient
	GetBalance(context.Context, *corev1.GetBalanceRequest) (*corev1.GetBalanceResponse, error)
}

type balanceResponse struct {
	UserID         int64  `json:"user_id"`
	Currency       string `json:"currency"`
	NetAmountMinor int64  `json:"net_amount_minor"`
}

func getBalance(core ledgerClient) http.HandlerFunc {
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

func balanceToResponse(balance *corev1.Balance) balanceResponse {
	if balance == nil {
		return balanceResponse{}
	}
	return balanceResponse{UserID: balance.UserId, Currency: balance.Currency, NetAmountMinor: balance.NetAmountMinor}
}
