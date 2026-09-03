package service

import (
	"context"
	"delim/internal/core/domain"
	corev1 "delim/pkg/gen/core/v1"
)

type AdjustmentService interface {
	Create(context.Context, int64, domain.Adjustment) (domain.Adjustment, error)
	List(context.Context, int64, int64) ([]domain.Adjustment, error)
}

func (s *GRPCServer) ListAdjustments(ctx context.Context, req *corev1.ListAdjustmentsRequest) (*corev1.ListAdjustmentsResponse, error) {
	values, err := s.adjustments.List(ctx, req.GetActorUserId(), req.GetExpenseId())
	if err != nil {
		return nil, toGRPCError(err)
	}
	response := &corev1.ListAdjustmentsResponse{}
	for _, value := range values {
		response.Adjustments = append(response.Adjustments, adjustmentToProto(value))
	}
	return response, nil
}

func (s *GRPCServer) CreateAdjustment(ctx context.Context, req *corev1.CreateAdjustmentRequest) (*corev1.CreateAdjustmentResponse, error) {
	value := domain.Adjustment{ExpenseID: req.GetExpenseId(), Type: adjustmentTypeFromProto(req.GetType()), AmountMinor: req.GetAmountMinor(), Currency: req.GetCurrency()}
	for _, allocation := range req.GetAllocations() {
		value.Allocations = append(value.Allocations, domain.AdjustmentAllocation{UserID: allocation.GetUserId(), AmountMinor: allocation.GetAmountMinor()})
	}
	saved, err := s.adjustments.Create(ctx, req.GetActorUserId(), value)
	if err != nil {
		return nil, toGRPCError(err)
	}
	return &corev1.CreateAdjustmentResponse{Adjustment: adjustmentToProto(saved)}, nil
}
func adjustmentTypeFromProto(value corev1.AdjustmentType) domain.AdjustmentType {
	if value == corev1.AdjustmentType_ADJUSTMENT_TYPE_REFUND {
		return domain.AdjustmentRefund
	}
	if value == corev1.AdjustmentType_ADJUSTMENT_TYPE_CORRECTION {
		return domain.AdjustmentCorrection
	}
	return ""
}
func adjustmentTypeToProto(value domain.AdjustmentType) corev1.AdjustmentType {
	if value == domain.AdjustmentRefund {
		return corev1.AdjustmentType_ADJUSTMENT_TYPE_REFUND
	}
	if value == domain.AdjustmentCorrection {
		return corev1.AdjustmentType_ADJUSTMENT_TYPE_CORRECTION
	}
	return corev1.AdjustmentType_ADJUSTMENT_TYPE_UNSPECIFIED
}
func adjustmentToProto(value domain.Adjustment) *corev1.Adjustment {
	result := &corev1.Adjustment{Id: value.ID, GroupId: value.GroupID, ExpenseId: value.ExpenseID, Type: adjustmentTypeToProto(value.Type), AmountMinor: value.AmountMinor, Currency: value.Currency, CreatedBy: value.CreatedBy, CreatedAtUnix: value.CreatedAt.Unix()}
	for _, allocation := range value.Allocations {
		result.Allocations = append(result.Allocations, &corev1.AdjustmentAllocation{UserId: allocation.UserID, AmountMinor: allocation.AmountMinor})
	}
	return result
}
