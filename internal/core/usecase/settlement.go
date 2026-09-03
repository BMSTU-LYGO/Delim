package usecase

import (
	"context"
	"delim/internal/core/domain"
)

type SettlementRepository interface {
	CreateSettlement(context.Context, int64, domain.Settlement) (domain.Settlement, error)
}
type Settlements struct{ repository SettlementRepository }

func NewSettlements(repository SettlementRepository) *Settlements {
	return &Settlements{repository: repository}
}
func (s *Settlements) Create(ctx context.Context, actorID int64, input domain.Settlement) (domain.Settlement, error) {
	if actorID <= 0 || input.GroupID <= 0 || input.SenderUserID <= 0 || input.ReceiverUserID <= 0 || input.SenderUserID == input.ReceiverUserID || input.AmountMinor <= 0 || !validCurrency(input.Currency) {
		return domain.Settlement{}, domain.ErrInvalidArgument
	}
	return s.repository.CreateSettlement(ctx, actorID, input)
}
