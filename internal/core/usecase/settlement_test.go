package usecase

import (
	"delim/internal/core/domain"
	"errors"
	"testing"
)

func TestSettlementCreatePermissions(t *testing.T) {
	valid := domain.Settlement{GroupID: 1, SenderUserID: 1, ReceiverUserID: 2, AmountMinor: 100, Currency: "RUB"}
	tests := []struct {
		name   string
		actor  int64
		mutate func(*domain.Settlement)
		want   error
	}{{"sender creates", 1, func(*domain.Settlement) {}, nil}, {"third party forbidden", 3, func(*domain.Settlement) {}, domain.ErrForbidden}, {"receiver cannot create for sender", 2, func(*domain.Settlement) {}, domain.ErrForbidden}, {"same party", 1, func(value *domain.Settlement) { value.ReceiverUserID = 1 }, domain.ErrInvalidArgument}, {"non-positive amount", 1, func(value *domain.Settlement) { value.AmountMinor = 0 }, domain.ErrInvalidArgument}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			value := valid
			tt.mutate(&value)
			if got := validateSettlementCreate(tt.actor, value); !errors.Is(got, tt.want) {
				t.Fatalf("got %v want %v", got, tt.want)
			}
		})
	}
}
