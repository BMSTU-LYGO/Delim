package usecase

import (
	"context"
	"strings"

	"delim/internal/core/domain"
)

type GroupRepository interface {
	CreateGroup(context.Context, int64, string) (domain.Group, error)
}
type Groups struct{ repository GroupRepository }

func NewGroups(repository GroupRepository) *Groups { return &Groups{repository: repository} }
func (g *Groups) Create(ctx context.Context, actorID int64, name string) (domain.Group, error) {
	name = strings.TrimSpace(name)
	if actorID <= 0 || name == "" {
		return domain.Group{}, domain.ErrInvalidArgument
	}
	return g.repository.CreateGroup(ctx, actorID, name)
}
