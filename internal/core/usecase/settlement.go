package usecase

import (
	"context"
	"delim/internal/core/domain"
)

type SettlementRepository interface {
	CreateSettlement(context.Context, int64, domain.Settlement) (domain.Settlement, error)
	ConfirmSettlement(context.Context, int64, int64) (domain.Settlement, error)
	ListSettlements(context.Context, int64, int64, int64, int32) ([]domain.Settlement, error)
}

func (s *Settlements) List(ctx context.Context, actorID, groupID, cursor int64, limit int32) ([]domain.Settlement, error) {
	if actorID <= 0 || groupID <= 0 || cursor < 0 {
		return nil, domain.ErrInvalidArgument
	}
	if limit == 0 {
		limit = 50
	}
	if limit < 0 || limit > 100 {
		return nil, domain.ErrInvalidArgument
	}
	return s.repository.ListSettlements(ctx, actorID, groupID, cursor, limit)
}

func (s *Settlements) Confirm(ctx context.Context, actorID, settlementID int64) (domain.Settlement, error) {
	if actorID <= 0 || settlementID <= 0 {
		return domain.Settlement{}, domain.ErrInvalidArgument
	}
	return s.repository.ConfirmSettlement(ctx, actorID, settlementID)
}

type Settlements struct{ repository SettlementRepository }

func NewSettlements(repository SettlementRepository) *Settlements {
	return &Settlements{repository: repository}
}
func (s *Settlements) Create(ctx context.Context, actorID int64, input domain.Settlement) (domain.Settlement, error) {
	if err := validateSettlementCreate(actorID, input); err != nil {
		return domain.Settlement{}, err
	}
	return s.repository.CreateSettlement(ctx, actorID, input)
}

func validateSettlementCreate(actorID int64, input domain.Settlement) error {
	if actorID <= 0 || input.GroupID <= 0 || input.SenderUserID <= 0 || input.ReceiverUserID <= 0 || input.SenderUserID == input.ReceiverUserID || input.AmountMinor <= 0 || !validCurrency(input.Currency) {
		return domain.ErrInvalidArgument
	}
	if actorID != input.SenderUserID {
		return domain.ErrForbidden
	}
	return nil
}
