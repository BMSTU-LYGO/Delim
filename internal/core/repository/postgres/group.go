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

func (s *Store) GetGroup(ctx context.Context, actorID, groupID int64) (domain.Group, error) {
	var group domain.Group
	err := s.pool.QueryRow(ctx, `SELECT g.id,g.name,g.owner_id,g.status,g.created_at,g.updated_at,gm.role FROM groups g JOIN group_members gm ON gm.group_id=g.id AND gm.user_id=$1 WHERE g.id=$2`, actorID, groupID).
		Scan(&group.ID, &group.Name, &group.OwnerID, &group.Status, &group.CreatedAt, &group.UpdatedAt, &group.CurrentUserRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Group{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Group{}, err
	}
	return group, nil
}

func (s *Store) ListGroups(ctx context.Context, actorID, cursor int64, limit int32) ([]domain.Group, error) {
	rows, err := s.pool.Query(ctx, `SELECT g.id,g.name,g.owner_id,g.status,g.created_at,g.updated_at,gm.role FROM group_members gm JOIN groups g ON g.id=gm.group_id WHERE gm.user_id=$1 AND ($2=0 OR g.id<$2) ORDER BY g.id DESC LIMIT $3`, actorID, cursor, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := make([]domain.Group, 0, limit)
	for rows.Next() {
		var group domain.Group
		if err := rows.Scan(&group.ID, &group.Name, &group.OwnerID, &group.Status, &group.CreatedAt, &group.UpdatedAt, &group.CurrentUserRole); err != nil {
			return nil, err
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return groups, nil
}

func (s *Store) JoinGroup(ctx context.Context, actorID, groupID int64) (domain.GroupMember, error) {
	var member domain.GroupMember
	err := s.pool.QueryRow(ctx, `
		INSERT INTO group_members(group_id,user_id,role)
		SELECT g.id,$1,'member' FROM groups g JOIN users u ON u.id=$1 WHERE g.id=$2 AND g.status='active'
		ON CONFLICT(group_id,user_id) DO UPDATE SET user_id=EXCLUDED.user_id
		RETURNING group_id,user_id,role,joined_at`, actorID, groupID).
		Scan(&member.GroupID, &member.UserID, &member.Role, &member.JoinedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GroupMember{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.GroupMember{}, err
	}
	return member, nil
}

func (s *Store) UpdateMemberRole(ctx context.Context, actorID, groupID, userID int64, role domain.MemberRole) (domain.GroupMember, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.GroupMember{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status domain.GroupStatus
	var actorRole, targetRole domain.MemberRole
	err = tx.QueryRow(ctx, `SELECT g.status,a.role,t.role FROM groups g JOIN group_members a ON a.group_id=g.id AND a.user_id=$1 JOIN group_members t ON t.group_id=g.id AND t.user_id=$2 WHERE g.id=$3 FOR UPDATE OF g`, actorID, userID, groupID).Scan(&status, &actorRole, &targetRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.GroupMember{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.GroupMember{}, err
	}
	if status == domain.GroupArchived {
		return domain.GroupMember{}, domain.ErrArchivedGroup
	}
	if actorRole != domain.RoleOwner && actorRole != domain.RoleAdmin {
		return domain.GroupMember{}, domain.ErrForbidden
	}
	if targetRole == domain.RoleOwner || role == domain.RoleOwner {
		return domain.GroupMember{}, domain.ErrForbidden
	}
	var member domain.GroupMember
	err = tx.QueryRow(ctx, `UPDATE group_members SET role=$1 WHERE group_id=$2 AND user_id=$3 RETURNING group_id,user_id,role,joined_at`, role, groupID, userID).Scan(&member.GroupID, &member.UserID, &member.Role, &member.JoinedAt)
	if err != nil {
		return domain.GroupMember{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(group_id,actor_user_id,action,entity_type,entity_id,metadata) VALUES($1,$2,'group.member_role_updated','group_member',$3,jsonb_build_object('role',$4::text))`, groupID, actorID, userID, role); err != nil {
		return domain.GroupMember{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.GroupMember{}, err
	}
	return member, nil
}

func (s *Store) ArchiveGroup(ctx context.Context, actorID, groupID int64) (domain.Group, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Group{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var group domain.Group
	err = tx.QueryRow(ctx, `SELECT g.id,g.name,g.owner_id,g.status,g.created_at,g.updated_at,gm.role FROM groups g JOIN group_members gm ON gm.group_id=g.id AND gm.user_id=$1 WHERE g.id=$2 FOR UPDATE OF g`, actorID, groupID).
		Scan(&group.ID, &group.Name, &group.OwnerID, &group.Status, &group.CreatedAt, &group.UpdatedAt, &group.CurrentUserRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Group{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Group{}, err
	}
	if group.CurrentUserRole != domain.RoleOwner && group.CurrentUserRole != domain.RoleAdmin {
		return domain.Group{}, domain.ErrForbidden
	}
	if group.Status == domain.GroupArchived {
		return group, tx.Commit(ctx)
	}
	err = tx.QueryRow(ctx, `UPDATE groups SET status='archived',updated_at=NOW() WHERE id=$1 RETURNING status,updated_at`, groupID).Scan(&group.Status, &group.UpdatedAt)
	if err != nil {
		return domain.Group{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(group_id,actor_user_id,action,entity_type,entity_id,entity_version,metadata) VALUES($1,$2,'group.archived','group',$1,1,'{}')`, groupID, actorID); err != nil {
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
