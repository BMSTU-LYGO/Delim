package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

type StoredUpdate struct {
	EventKey string
	Payload  []byte
	Attempts int
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

func (s *Store) ClaimUpdates(ctx context.Context, limit int, leaseUntil time.Time) ([]StoredUpdate, error) {
	const query = `
		WITH picked AS (
			SELECT event_key
			FROM gateway_max_updates
			WHERE (status = 'pending' OR status = 'processing')
			  AND next_attempt_at <= NOW()
			ORDER BY next_attempt_at, received_at
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE gateway_max_updates AS updates
		SET status = 'processing', attempts = updates.attempts + 1, next_attempt_at = $2
		FROM picked
		WHERE updates.event_key = picked.event_key
		RETURNING updates.event_key, updates.payload, updates.attempts`
	rows, err := s.pool.Query(ctx, query, limit, leaseUntil)
	if err != nil {
		return nil, fmt.Errorf("claim MAX updates: %w", err)
	}
	defer rows.Close()

	updates := make([]StoredUpdate, 0, limit)
	for rows.Next() {
		var update StoredUpdate
		if err := rows.Scan(&update.EventKey, &update.Payload, &update.Attempts); err != nil {
			return nil, fmt.Errorf("scan MAX update: %w", err)
		}
		updates = append(updates, update)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate MAX updates: %w", err)
	}
	return updates, nil
}

func (s *Store) CompleteUpdate(ctx context.Context, eventKey string) error {
	const query = `
		UPDATE gateway_max_updates
		SET status = 'processed', processed_at = NOW(), last_error = NULL, payload = '{}'::jsonb
		WHERE event_key = $1 AND status = 'processing'`
	if _, err := s.pool.Exec(ctx, query, eventKey); err != nil {
		return fmt.Errorf("complete MAX update: %w", err)
	}
	return nil
}

func (s *Store) FailUpdate(ctx context.Context, eventKey, lastError string, nextAttemptAt time.Time, terminal bool) error {
	status := "pending"
	if terminal {
		status = "failed"
	}
	const query = `
		UPDATE gateway_max_updates
		SET status = $2, next_attempt_at = $3, last_error = $4
		WHERE event_key = $1 AND status = 'processing'`
	if _, err := s.pool.Exec(ctx, query, eventKey, status, nextAttemptAt, lastError); err != nil {
		return fmt.Errorf("fail MAX update: %w", err)
	}
	return nil
}
