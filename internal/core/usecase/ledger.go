package usecase

import (
	"context"
	"delim/internal/core/domain"
)

type LedgerRepository interface {
	GetBalance(context.Context, int64, int64) ([]domain.Balance, error)
	GetBalanceBreakdown(context.Context, int64, int64, int64) ([]domain.BalanceEntry, []domain.Balance, error)
}

func (l *Ledger) GetBalanceBreakdown(ctx context.Context, actorID, groupID, userID int64) ([]domain.BalanceEntry, []domain.Balance, error) {
	if actorID <= 0 || groupID <= 0 || userID <= 0 {
		return nil, nil, domain.ErrInvalidArgument
	}
	return l.repository.GetBalanceBreakdown(ctx, actorID, groupID, userID)
}

type Ledger struct{ repository LedgerRepository }

func NewLedger(repository LedgerRepository) *Ledger { return &Ledger{repository: repository} }
func (l *Ledger) GetBalance(ctx context.Context, actorID, groupID int64) ([]domain.Balance, error) {
	if actorID <= 0 || groupID <= 0 {
		return nil, domain.ErrInvalidArgument
	}
	return l.repository.GetBalance(ctx, actorID, groupID)
}
