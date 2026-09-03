package postgres

import (
	"context"
	"errors"

	"delim/internal/core/domain"
	"github.com/jackc/pgx/v5"
)

func (s *Store) UpsertUser(ctx context.Context, user domain.User) (domain.User, error) {
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (max_user_id, first_name, last_name, username)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (max_user_id) DO UPDATE SET
			first_name = EXCLUDED.first_name,
			last_name = EXCLUDED.last_name,
			username = EXCLUDED.username,
			updated_at = NOW()
		RETURNING id, max_user_id, first_name, last_name, username, created_at, updated_at`,
		user.MaxUserID, user.FirstName, user.LastName, user.Username,
	).Scan(&user.ID, &user.MaxUserID, &user.FirstName, &user.LastName, &user.Username, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}

func (s *Store) GetUser(ctx context.Context, id int64) (domain.User, error) {
	var user domain.User
	err := s.pool.QueryRow(ctx, `SELECT id, max_user_id, first_name, last_name, username, created_at, updated_at FROM users WHERE id = $1`, id).
		Scan(&user.ID, &user.MaxUserID, &user.FirstName, &user.LastName, &user.Username, &user.CreatedAt, &user.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.User{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.User{}, err
	}
	return user, nil
}
