package usecase

import (
	"context"
	"strings"

	"delim/internal/core/domain"
)

type UserRepository interface {
	UpsertUser(context.Context, domain.User) (domain.User, error)
	GetUser(context.Context, int64) (domain.User, error)
}

type Users struct{ repository UserRepository }

func NewUsers(repository UserRepository) *Users { return &Users{repository: repository} }

func (u *Users) Upsert(ctx context.Context, user domain.User) (domain.User, error) {
	if user.MaxUserID <= 0 {
		return domain.User{}, domain.ErrInvalidArgument
	}
	user.FirstName = strings.TrimSpace(user.FirstName)
	user.LastName = strings.TrimSpace(user.LastName)
	user.Username = strings.TrimSpace(user.Username)
	return u.repository.UpsertUser(ctx, user)
}

func (u *Users) Get(ctx context.Context, id int64) (domain.User, error) {
	if id <= 0 {
		return domain.User{}, domain.ErrInvalidArgument
	}
	return u.repository.GetUser(ctx, id)
}
