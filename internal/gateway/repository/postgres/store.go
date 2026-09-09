package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

func (s *Store) UpsertChat(ctx context.Context, chatID int64, isChannel bool, status string, eventAt time.Time) error {
	if chatID == 0 {
		return fmt.Errorf("upsert MAX chat: missing chat_id")
	}
	const query = `
		INSERT INTO gateway_max_chats (chat_id, is_channel, status, last_event_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (chat_id) DO UPDATE
		SET is_channel = EXCLUDED.is_channel,
		    status = EXCLUDED.status,
		    last_event_at = EXCLUDED.last_event_at,
		    updated_at = NOW()
		WHERE EXCLUDED.last_event_at >= gateway_max_chats.last_event_at`
	if _, err := s.pool.Exec(ctx, query, chatID, isChannel, status, eventAt); err != nil {
		return fmt.Errorf("upsert MAX chat: %w", err)
	}
	return nil
}

func (s *Store) IsChatActive(ctx context.Context, chatID int64) (bool, error) {
	const query = `SELECT status FROM gateway_max_chats WHERE chat_id = $1`
	var status string
	if err := s.pool.QueryRow(ctx, query, chatID).Scan(&status); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("get MAX chat status: %w", err)
	}
	return status == "active", nil
}

// ChatGroupBinding links one active MAX chat to one Delim group (Block 1).
type ChatGroupBinding struct {
	ChatID        int64
	GroupID       int64
	BoundByUserID int64
	Status        string
}

var (
	ErrBindingNotFound   = errors.New("MAX chat binding not found")
	ErrGroupAlreadyBound = errors.New("Delim group is already bound to another MAX chat")
)

// BindChatGroup binds a MAX chat to a Delim group. Re-binding the same
// chat/group pair is idempotent. A group bound to a different chat is rejected
// with ErrGroupAlreadyBound.
func (s *Store) BindChatGroup(ctx context.Context, chatID, groupID, boundByUserID int64) (ChatGroupBinding, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return ChatGroupBinding{}, fmt.Errorf("begin chat group bind: %w", err)
	}
	defer tx.Rollback(ctx)

	var holder int64
	if err := tx.QueryRow(ctx,
		`SELECT chat_id FROM gateway_max_chat_groups WHERE group_id = $1 AND status = 'active'`, groupID,
	).Scan(&holder); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return ChatGroupBinding{}, fmt.Errorf("check group binding: %w", err)
	} else if err == nil && holder != chatID {
		return ChatGroupBinding{}, ErrGroupAlreadyBound
	}

	var binding ChatGroupBinding
	err = tx.QueryRow(ctx, `
		INSERT INTO gateway_max_chat_groups (chat_id, group_id, bound_by_user_id, status)
		VALUES ($1, $2, $3, 'active')
		ON CONFLICT (chat_id) DO UPDATE
		SET group_id = EXCLUDED.group_id,
		    bound_by_user_id = EXCLUDED.bound_by_user_id,
		    status = 'active',
		    updated_at = NOW()
		RETURNING chat_id, group_id, bound_by_user_id, status`,
		chatID, groupID, boundByUserID,
	).Scan(&binding.ChatID, &binding.GroupID, &binding.BoundByUserID, &binding.Status)
	if err != nil {
		if isUniqueViolation(err) {
			return ChatGroupBinding{}, ErrGroupAlreadyBound
		}
		return ChatGroupBinding{}, fmt.Errorf("bind MAX chat group: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return ChatGroupBinding{}, fmt.Errorf("commit chat group bind: %w", err)
	}
	return binding, nil
}

// GetGroupByChat returns the active Delim group bound to a MAX chat.
func (s *Store) GetGroupByChat(ctx context.Context, chatID int64) (int64, error) {
	var groupID int64
	err := s.pool.QueryRow(ctx,
		`SELECT group_id FROM gateway_max_chat_groups WHERE chat_id = $1 AND status = 'active'`, chatID,
	).Scan(&groupID)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, ErrBindingNotFound
	}
	if err != nil {
		return 0, fmt.Errorf("get group by chat: %w", err)
	}
	return groupID, nil
}

// GetChatByGroup returns the active MAX chat bound to a Delim group.
func (s *Store) GetChatByGroup(ctx context.Context, groupID int64) (ChatGroupBinding, error) {
	var binding ChatGroupBinding
	err := s.pool.QueryRow(ctx, `
		SELECT chat_id, group_id, bound_by_user_id, status
		FROM gateway_max_chat_groups
		WHERE group_id = $1 AND status = 'active'`, groupID,
	).Scan(&binding.ChatID, &binding.GroupID, &binding.BoundByUserID, &binding.Status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChatGroupBinding{}, ErrBindingNotFound
	}
	if err != nil {
		return ChatGroupBinding{}, fmt.Errorf("get chat by group: %w", err)
	}
	return binding, nil
}

// UnbindChatGroup marks the chat/group binding unbound (idempotent). The row is
// kept so binding history and audit remain available.
func (s *Store) UnbindChatGroup(ctx context.Context, chatID int64) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE gateway_max_chat_groups
		SET status = 'unbound', updated_at = NOW()
		WHERE chat_id = $1 AND status = 'active'`, chatID)
	if err != nil {
		return fmt.Errorf("unbind MAX chat group: %w", err)
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

// StoredNotification is a durable outbound MAX notification (Block 5).
type StoredNotification struct {
	ID        int64
	DedupeKey string
	ChatID    int64
	Kind      string
	Payload   []byte
	Attempts  int
}

// EnqueueNotification persists a notification; a repeated dedupe_key is
// idempotently ignored (returns false).
func (s *Store) EnqueueNotification(ctx context.Context, dedupeKey, kind string, chatID int64, payload []byte) (bool, error) {
	const query = `
		INSERT INTO gateway_notifications (dedupe_key, chat_id, kind, payload)
		VALUES ($1, $2, $3, $4::jsonb)
		ON CONFLICT (dedupe_key) DO NOTHING`
	tag, err := s.pool.Exec(ctx, query, dedupeKey, chatID, kind, payload)
	if err != nil {
		return false, fmt.Errorf("enqueue notification: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ClaimNotifications leases due pending notifications with SKIP LOCKED.
func (s *Store) ClaimNotifications(ctx context.Context, limit int, leaseUntil time.Time) ([]StoredNotification, error) {
	const query = `
		WITH picked AS (
			SELECT id
			FROM gateway_notifications
			WHERE status = 'pending' AND next_attempt_at <= NOW()
			ORDER BY next_attempt_at, id
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE gateway_notifications AS notifications
		SET status = 'processing', attempts = notifications.attempts + 1, next_attempt_at = $2
		FROM picked
		WHERE notifications.id = picked.id
		RETURNING notifications.id, notifications.dedupe_key, notifications.chat_id,
		          notifications.kind, notifications.payload, notifications.attempts`
	rows, err := s.pool.Query(ctx, query, limit, leaseUntil)
	if err != nil {
		return nil, fmt.Errorf("claim notifications: %w", err)
	}
	defer rows.Close()
	notifications := make([]StoredNotification, 0, limit)
	for rows.Next() {
		var notification StoredNotification
		if err := rows.Scan(&notification.ID, &notification.DedupeKey, &notification.ChatID,
			&notification.Kind, &notification.Payload, &notification.Attempts); err != nil {
			return nil, fmt.Errorf("scan notification: %w", err)
		}
		notifications = append(notifications, notification)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate notifications: %w", err)
	}
	return notifications, nil
}

// CompleteNotification marks a notification as sent.
func (s *Store) CompleteNotification(ctx context.Context, id int64) error {
	const query = `
		UPDATE gateway_notifications
		SET status = 'sent', sent_at = NOW(), last_error = NULL
		WHERE id = $1 AND status = 'processing'`
	if _, err := s.pool.Exec(ctx, query, id); err != nil {
		return fmt.Errorf("complete notification: %w", err)
	}
	return nil
}

// FailNotification reschedules or marks a notification failed.
func (s *Store) FailNotification(ctx context.Context, id int64, lastError string, nextAttemptAt time.Time, terminal bool) error {
	status := "pending"
	if terminal {
		status = "failed"
	}
	const query = `
		UPDATE gateway_notifications
		SET status = $2, next_attempt_at = $3, last_error = $4
		WHERE id = $1 AND status = 'processing'`
	if _, err := s.pool.Exec(ctx, query, id, status, nextAttemptAt, lastError); err != nil {
		return fmt.Errorf("fail notification: %w", err)
	}
	return nil
}

// CountPendingNotifications reports how many notifications are still pending.
func (s *Store) CountPendingNotifications(ctx context.Context) (int64, error) {
	var count int64
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM gateway_notifications WHERE status = 'pending'`).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count pending notifications: %w", err)
	}
	return count, nil
}
