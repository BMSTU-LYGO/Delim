package service

import (
	"context"
	"delim/internal/core/domain"
	corev1 "delim/pkg/gen/core/v1"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type SettlementService interface {
	Create(context.Context, int64, domain.Settlement) (domain.Settlement, error)
	Confirm(context.Context, int64, int64) (domain.Settlement, error)
	List(context.Context, int64, int64, int64, int32) ([]domain.Settlement, error)
}

func (s *GRPCServer) ListSettlements(ctx context.Context, req *corev1.ListSettlementsRequest) (*corev1.ListSettlementsResponse, error) {
	var cursor int64
	var limit int32
	if req.GetPage() != nil {
		cursor, limit = req.GetPage().GetCursorId(), req.GetPage().GetLimit()
	}
	values, err := s.settlements.List(ctx, req.GetActorUserId(), req.GetGroupId(), cursor, limit)
	if err != nil {
		return nil, toGRPCError(err)
	}
	response := &corev1.ListSettlementsResponse{Page: &corev1.PageResponse{}}
	for _, value := range values {
		response.Settlements = append(response.Settlements, settlementToProto(value))
	}
	if len(values) > 0 {
		response.Page.NextCursorId = values[len(values)-1].ID
	}
	return response, nil
}

func (s *GRPCServer) ConfirmSettlement(ctx context.Context, req *corev1.ConfirmSettlementRequest) (*corev1.ConfirmSettlementResponse, error) {
	settlement, err := s.settlements.Confirm(ctx, req.GetActorUserId(), req.GetSettlementId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.ConfirmSettlementResponse{Settlement: settlementToProto(settlement)}, nil
}

func (s *GRPCServer) CreateSettlement(ctx context.Context, req *corev1.CreateSettlementRequest) (*corev1.CreateSettlementResponse, error) {
	settlement, err := s.settlements.Create(ctx, req.GetActorUserId(), domain.Settlement{GroupID: req.GetGroupId(), SenderUserID: req.GetSenderUserId(), ReceiverUserID: req.GetReceiverUserId(), AmountMinor: req.GetAmountMinor(), Currency: req.GetCurrency()})
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.CreateSettlementResponse{Settlement: settlementToProto(settlement)}, nil
}
func settlementToProto(value domain.Settlement) *corev1.Settlement {
	result := &corev1.Settlement{Id: value.ID, GroupId: value.GroupID, SenderUserId: value.SenderUserID, ReceiverUserId: value.ReceiverUserID, AmountMinor: value.AmountMinor, Currency: value.Currency, Status: settlementStatusToProto(value.Status), CreatedBy: value.CreatedBy, Version: value.Version, CreatedAt: timestamppb.New(value.CreatedAt)}
	if value.ConfirmedAt != nil {
		result.ConfirmedAt = timestamppb.New(*value.ConfirmedAt)
	}
	return result
}
func settlementStatusToProto(value domain.SettlementStatus) corev1.SettlementStatus {
	switch value {
	case domain.SettlementPending:
		return corev1.SettlementStatus_SETTLEMENT_STATUS_PENDING
	case domain.SettlementConfirmed:
		return corev1.SettlementStatus_SETTLEMENT_STATUS_CONFIRMED
	case domain.SettlementCancelled:
		return corev1.SettlementStatus_SETTLEMENT_STATUS_CANCELLED
	default:
		return corev1.SettlementStatus_SETTLEMENT_STATUS_UNSPECIFIED
	}
}
