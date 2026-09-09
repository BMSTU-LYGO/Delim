package usecase

import (
	"context"
	"delim/internal/core/domain"
)

type AdjustmentRepository interface {
	CreateAdjustment(context.Context, int64, domain.Adjustment) (domain.Adjustment, error)
	ListAdjustments(context.Context, int64, int64) ([]domain.Adjustment, error)
	ListGroupAdjustments(context.Context, int64, int64) ([]domain.Adjustment, error)
}

func (a *Adjustments) List(ctx context.Context, actorID, expenseID int64) ([]domain.Adjustment, error) {
	if actorID <= 0 || expenseID <= 0 {
		return nil, domain.ErrInvalidArgument
	}
	return a.repository.ListAdjustments(ctx, actorID, expenseID)
}

func (a *Adjustments) ListGroupAdjustments(ctx context.Context, actorID, groupID int64) ([]domain.Adjustment, error) {
	if actorID <= 0 || groupID <= 0 {
		return nil, domain.ErrInvalidArgument
	}
	return a.repository.ListGroupAdjustments(ctx, actorID, groupID)
}

type Adjustments struct{ repository AdjustmentRepository }

func NewAdjustments(repository AdjustmentRepository) *Adjustments {
	return &Adjustments{repository: repository}
}
func (a *Adjustments) Create(ctx context.Context, actorID int64, value domain.Adjustment) (domain.Adjustment, error) {
	if actorID <= 0 || value.ExpenseID <= 0 || value.AmountMinor <= 0 || (value.Type != domain.AdjustmentRefund && value.Type != domain.AdjustmentCorrection) || !validCurrency(value.Currency) {
		return domain.Adjustment{}, domain.ErrInvalidArgument
	}
	currency := value.Currency
	result, err := a.repository.CreateAdjustment(ctx, actorID, value)
	if err != nil {
		return domain.Adjustment{}, err
	}
	if result.Currency != currency {
		return domain.Adjustment{}, domain.ErrInvalidArgument
	}
	return result, nil
}
