package usecase

import (
	"context"
	"delim/internal/core/domain"
)

type AdjustmentRepository interface {
	CreateAdjustment(context.Context, int64, domain.Adjustment) (domain.Adjustment, error)
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
