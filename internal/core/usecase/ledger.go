package usecase

import (
	"context"
	"delim/internal/core/domain"
)

type LedgerRepository interface {
	LoadLedger(context.Context, int64, int64) (domain.LedgerInput, error)
	GetBalanceBreakdown(context.Context, int64, int64, int64) ([]domain.BalanceEntry, error)
}

func (l *Ledger) GetBalanceBreakdown(ctx context.Context, actorID, groupID, userID int64) ([]domain.BalanceEntry, []domain.Balance, error) {
	if actorID <= 0 || groupID <= 0 || userID <= 0 {
		return nil, nil, domain.ErrInvalidArgument
	}
	entries, err := l.repository.GetBalanceBreakdown(ctx, actorID, groupID, userID)
	if err != nil {
		return nil, nil, err
	}
	all, err := l.GetBalance(ctx, actorID, groupID)
	if err != nil {
		return nil, nil, err
	}
	balances := make([]domain.Balance, 0)
	for _, balance := range all {
		if balance.UserID == userID {
			balances = append(balances, balance)
		}
	}
	return entries, balances, nil
}

func (l *Ledger) GetSettlementPlan(ctx context.Context, actorID, groupID int64) ([]domain.SettlementPlanTransfer, error) {
	balances, err := l.GetBalance(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	return domain.PlanSettlements(balances)
}

type Ledger struct{ repository LedgerRepository }

func NewLedger(repository LedgerRepository) *Ledger { return &Ledger{repository: repository} }
func (l *Ledger) GetBalance(ctx context.Context, actorID, groupID int64) ([]domain.Balance, error) {
	if actorID <= 0 || groupID <= 0 {
		return nil, domain.ErrInvalidArgument
	}
	input, err := l.repository.LoadLedger(ctx, actorID, groupID)
	if err != nil {
		return nil, err
	}
	return domain.CalculateBalances(input)
}
