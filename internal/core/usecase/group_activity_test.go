package usecase

import (
	"context"
	"errors"
	"testing"

	"delim/internal/core/domain"
)

type groupActivityRepository struct {
	created domain.GroupInput
}

func (r *groupActivityRepository) CreateGroup(_ context.Context, _ int64, input domain.GroupInput) (domain.Group, error) {
	r.created = input
	return domain.Group{Name: input.Name}, nil
}
func (r *groupActivityRepository) GetGroupBudgetSummary(context.Context, int64, int64) (domain.GroupBudgetSummary, error) {
	return domain.GroupBudgetSummary{}, nil
}
func (r *groupActivityRepository) GetGroup(context.Context, int64, int64) (domain.Group, error) {
	return domain.Group{}, nil
}
func (r *groupActivityRepository) ListGroups(context.Context, int64, int64, int32) ([]domain.Group, error) {
	return nil, nil
}
func (r *groupActivityRepository) ListGroupMembers(context.Context, int64, int64) ([]domain.GroupMember, error) {
	return nil, nil
}
func (r *groupActivityRepository) AddGroupMembers(context.Context, int64, int64, []int64) ([]domain.GroupMember, error) {
	return nil, nil
}
func (r *groupActivityRepository) JoinGroup(context.Context, int64, int64) (domain.GroupMember, error) {
	return domain.GroupMember{}, nil
}
func (r *groupActivityRepository) UpdateMemberRole(context.Context, int64, int64, int64, domain.MemberRole) (domain.GroupMember, error) {
	return domain.GroupMember{}, nil
}
func (r *groupActivityRepository) ArchiveGroup(context.Context, int64, int64) (domain.Group, error) {
	return domain.Group{}, nil
}

func TestGroupActivityInputValidation(t *testing.T) {
	budget := int64(42_000)
	for _, test := range []struct {
		name  string
		input domain.GroupInput
		valid bool
	}{
		{"valid trip", domain.GroupInput{ActivityType: "trip", StartDate: "2026-06-01", EndDate: "2026-06-03", PlannedBudgetMinor: &budget}, true},
		{"unknown activity", domain.GroupInput{ActivityType: "party"}, false},
		{"one date", domain.GroupInput{StartDate: "2026-06-01"}, false},
		{"end before start", domain.GroupInput{StartDate: "2026-06-03", EndDate: "2026-06-01"}, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			if got := validGroupInput(test.input); got != test.valid {
				t.Fatalf("validGroupInput() = %v, want %v", got, test.valid)
			}
		})
	}
}

func TestCreatePersistsTrimmedActivityInput(t *testing.T) {
	repository := &groupActivityRepository{}
	budget := int64(90_000)
	_, err := NewGroups(repository).Create(context.Background(), 7, domain.GroupInput{Name: "  Weekend  ", ActivityType: " trip ", Location: "  Казань ", StartDate: "2026-06-01", EndDate: "2026-06-03", PlannedBudgetMinor: &budget})
	if err != nil {
		t.Fatal(err)
	}
	if repository.created.Name != "Weekend" || repository.created.ActivityType != "trip" || repository.created.Location != "Казань" || repository.created.PlannedBudgetMinor != &budget {
		t.Fatalf("unexpected repository input: %#v", repository.created)
	}
	_, err = NewGroups(repository).Create(context.Background(), 7, domain.GroupInput{Name: "Bad", ActivityType: "other"})
	if !errors.Is(err, domain.ErrInvalidArgument) {
		t.Fatalf("invalid activity error = %v", err)
	}
}
