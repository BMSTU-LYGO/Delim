package postgres

import (
	"context"
	"errors"

	"delim/internal/core/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateGroup(ctx context.Context, actorID int64, name string) (domain.Group, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Group{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, actorID).Scan(&exists); err != nil {
		return domain.Group{}, err
	}
	if !exists {
		return domain.Group{}, domain.ErrNotFound
	}
	var group domain.Group
	err = tx.QueryRow(ctx, `INSERT INTO groups(name, owner_id) VALUES($1,$2) RETURNING id,name,owner_id,status,created_at,updated_at`, name, actorID).
		Scan(&group.ID, &group.Name, &group.OwnerID, &group.Status, &group.CreatedAt, &group.UpdatedAt)
	if err != nil {
		return domain.Group{}, err
	}
	group.CurrentUserRole = domain.RoleOwner
	if _, err = tx.Exec(ctx, `INSERT INTO group_members(group_id,user_id,role) VALUES($1,$2,'owner')`, group.ID, actorID); err != nil {
		return domain.Group{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(group_id,actor_user_id,action,entity_type,entity_id,entity_version,metadata) VALUES($1,$2,'group.created','group',$1,1,'{}')`, group.ID, actorID); err != nil {
		return domain.Group{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Group{}, err
	}
	return group, nil
}

func (s *Store) userExists(ctx context.Context, id int64) error {
	var found int64
	err := s.pool.QueryRow(ctx, `SELECT id FROM users WHERE id=$1`, id).Scan(&found)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	return err
}
