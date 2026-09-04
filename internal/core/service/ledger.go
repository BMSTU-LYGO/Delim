package service

import (
	"context"
	"delim/internal/core/domain"
	corev1 "delim/pkg/gen/core/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type LedgerService interface {
	GetBalance(context.Context, int64, int64) ([]domain.Balance, error)
	GetBalanceBreakdown(context.Context, int64, int64, int64) ([]domain.BalanceEntry, []domain.Balance, error)
	GetSettlementPlan(context.Context, int64, int64) ([]domain.SettlementPlanTransfer, error)
}

func (s *GRPCServer) GetSettlementPlan(ctx context.Context, req *corev1.GetSettlementPlanRequest) (*corev1.GetSettlementPlanResponse, error) {
	transfers, err := s.ledger.GetSettlementPlan(ctx, req.GetActorUserId(), req.GetGroupId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	response := &corev1.GetSettlementPlanResponse{}
	for _, transfer := range transfers {
		response.Transfers = append(response.Transfers, &corev1.SettlementPlanTransfer{FromUserId: transfer.FromUserID, ToUserId: transfer.ToUserID, AmountMinor: transfer.AmountMinor, Currency: transfer.Currency})
	}
	return response, nil
}

func (s *GRPCServer) GetBalanceBreakdown(ctx context.Context, req *corev1.GetBalanceBreakdownRequest) (*corev1.GetBalanceBreakdownResponse, error) {
	entries, balances, err := s.ledger.GetBalanceBreakdown(ctx, req.GetActorUserId(), req.GetGroupId(), req.GetUserId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	response := &corev1.GetBalanceBreakdownResponse{}
	for _, entry := range entries {
		response.Entries = append(response.Entries, &corev1.BalanceEntry{OperationType: entry.OperationType, OperationId: entry.OperationID, Currency: entry.Currency, AmountMinor: entry.AmountMinor, OccurredAt: timestamppb.New(entry.OccurredAt)})
	}
	for _, balance := range balances {
		response.Balances = append(response.Balances, &corev1.Balance{UserId: balance.UserID, Currency: balance.Currency, NetAmountMinor: balance.NetAmountMinor})
	}
	return response, nil
}

func (s *GRPCServer) GetBalance(ctx context.Context, req *corev1.GetBalanceRequest) (*corev1.GetBalanceResponse, error) {
	balances, err := s.ledger.GetBalance(ctx, req.GetActorUserId(), req.GetGroupId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	response := &corev1.GetBalanceResponse{}
	for _, balance := range balances {
		response.Balances = append(response.Balances, &corev1.Balance{UserId: balance.UserID, Currency: balance.Currency, NetAmountMinor: balance.NetAmountMinor})
	}
	return response, nil
}
