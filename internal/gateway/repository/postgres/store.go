package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) Ping(ctx context.Context) error {
	return s.pool.Ping(ctx)
}

func (s *Store) IngestUpdate(ctx context.Context, eventKey, updateType string, chatID *int64, payload []byte) error {
	const query = `
		INSERT INTO gateway_max_updates (event_key, update_type, chat_id, payload)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (event_key) DO NOTHING`
	if _, err := s.pool.Exec(ctx, query, eventKey, updateType, chatID, payload); err != nil {
		return fmt.Errorf("ingest MAX update: %w", err)
	}
	return nil
}
