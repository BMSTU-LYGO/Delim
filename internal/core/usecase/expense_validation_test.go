package usecase

import (
	"delim/internal/core/domain"
	"testing"
	"time"
)

func TestValidateExpenseInput(t *testing.T) {
	valid := domain.ExpenseInput{GroupID: 1, PayerUserID: 1, AmountMinor: 100, Currency: "RUB", ExpenseDate: time.Now(), SplitType: domain.SplitEqual, Participants: []domain.SplitParticipant{{UserID: 1}}}
	tests := []struct {
		name    string
		mutate  func(*domain.ExpenseInput)
		wantErr bool
	}{{"valid", func(*domain.ExpenseInput) {}, false}, {"zero date", func(input *domain.ExpenseInput) { input.ExpenseDate = time.Time{} }, true}, {"invalid split", func(input *domain.ExpenseInput) { input.SplitType = "other" }, true}, {"duplicate participants", func(input *domain.ExpenseInput) {
		input.Participants = []domain.SplitParticipant{{UserID: 1}, {UserID: 1}}
	}, true}, {"items with equal", func(input *domain.ExpenseInput) {
		input.Items = []domain.ExpenseItemInput{{Name: "x", AmountMinor: 100, ParticipantUserIDs: []int64{1}}}
	}, true}, {"participants with item", func(input *domain.ExpenseInput) {
		input.SplitType = domain.SplitItem
		input.Items = []domain.ExpenseItemInput{{Name: "x", AmountMinor: 100, ParticipantUserIDs: []int64{1}}}
	}, true}, {"duplicate item participants", func(input *domain.ExpenseInput) {
		input.SplitType = domain.SplitItem
		input.Participants = nil
		input.Items = []domain.ExpenseItemInput{{Name: "x", AmountMinor: 100, ParticipantUserIDs: []int64{1, 1}}}
	}, true}}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := valid
			input.Participants = append([]domain.SplitParticipant(nil), valid.Participants...)
			tt.mutate(&input)
			if got := validateExpenseInput(input); (got != nil) != tt.wantErr {
				t.Fatalf("error=%v", got)
			}
		})
	}
}
