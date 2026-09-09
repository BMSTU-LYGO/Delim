package service

import (
	"context"
	"errors"

	"delim/internal/core/domain"
	corev1 "delim/pkg/gen/core/v1"
	"delim/pkg/metricsx"
)

type GRPCServer struct {
	corev1.UnimplementedCoreServiceServer
	users       UserService
	groups      GroupService
	expenses    ExpenseService
	ledger      LedgerService
	settlements SettlementService
	adjustments AdjustmentService
	recorder    *metricsx.Recorder
}

func NewGRPCServer(users UserService, groups GroupService, expenses ExpenseService, ledger LedgerService, settlements SettlementService, adjustments AdjustmentService, recorder *metricsx.Recorder) *GRPCServer {
	return &GRPCServer{users: users, groups: groups, expenses: expenses, ledger: ledger, settlements: settlements, adjustments: adjustments, recorder: recorder}
}

// recordOutcome translates a domain error into a bounded metric observation.
// Only errors originating from the financial surface (expenses, settlements,
// adjustments, ledger) raise financial errors; conflicts raise conflict
// counters so dashboards can split business conflicts from generic errors.
func (s *GRPCServer) recordOutcome(operation string, err error) {
	if s.recorder == nil {
		return
	}
	if err == nil {
		return
	}
	switch {
	case errors.Is(err, domain.ErrConflict), errors.Is(err, domain.ErrInvalidState):
		s.recorder.ObserveConflict(operation)
	}
	switch {
	case errors.Is(err, domain.ErrConflict):
		s.recorder.ObserveFinancialError(operation, "conflict")
	case errors.Is(err, domain.ErrInvalidArgument):
		s.recorder.ObserveFinancialError(operation, "invalid_argument")
	case errors.Is(err, domain.ErrForbidden):
		s.recorder.ObserveFinancialError(operation, "forbidden")
	case errors.Is(err, domain.ErrArchivedGroup):
		s.recorder.ObserveFinancialError(operation, "archived")
	case errors.Is(err, domain.ErrInvalidState):
		s.recorder.ObserveFinancialError(operation, "invalid_state")
	case errors.Is(err, domain.ErrNotFound):
		s.recorder.ObserveFinancialError(operation, "not_found")
	default:
		s.recorder.ObserveFinancialError(operation, "internal")
	}
}

func (s *GRPCServer) Ping(context.Context, *corev1.PingRequest) (*corev1.PingResponse, error) {
	return &corev1.PingResponse{Status: "ok"}, nil
}
