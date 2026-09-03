package postgres

import (
	"context"
	"delim/internal/core/domain"
	"errors"
	"github.com/jackc/pgx/v5"
)

func (s *Store) CreateSettlement(ctx context.Context, actorID int64, input domain.Settlement) (domain.Settlement, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Settlement{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var status domain.GroupStatus
	err = tx.QueryRow(ctx, `SELECT status FROM groups WHERE id=$1 FOR SHARE`, input.GroupID).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Settlement{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Settlement{}, err
	}
	if status == domain.GroupArchived {
		return domain.Settlement{}, domain.ErrArchivedGroup
	}
	var count int
	err = tx.QueryRow(ctx, `SELECT COUNT(*) FROM group_members WHERE group_id=$1 AND user_id=ANY($2)`, input.GroupID, []int64{actorID, input.SenderUserID, input.ReceiverUserID}).Scan(&count)
	if err != nil {
		return domain.Settlement{}, err
	}
	required := 3
	if actorID == input.SenderUserID || actorID == input.ReceiverUserID {
		required = 2
	}
	if count != required {
		return domain.Settlement{}, domain.ErrForbidden
	}
	err = tx.QueryRow(ctx, `INSERT INTO settlements(group_id,sender_user_id,receiver_user_id,amount_minor,currency,created_by) VALUES($1,$2,$3,$4,$5,$6) RETURNING id,group_id,sender_user_id,receiver_user_id,amount_minor,currency,status,created_by,version,created_at,confirmed_at`, input.GroupID, input.SenderUserID, input.ReceiverUserID, input.AmountMinor, input.Currency, actorID).Scan(&input.ID, &input.GroupID, &input.SenderUserID, &input.ReceiverUserID, &input.AmountMinor, &input.Currency, &input.Status, &input.CreatedBy, &input.Version, &input.CreatedAt, &input.ConfirmedAt)
	if err != nil {
		return domain.Settlement{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(group_id,actor_user_id,action,entity_type,entity_id,entity_version,metadata) VALUES($1,$2,'settlement.created','settlement',$3,$4,'{}')`, input.GroupID, actorID, input.ID, input.Version); err != nil {
		return domain.Settlement{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Settlement{}, err
	}
	return input, nil
}

func (s *Store) ConfirmSettlement(ctx context.Context, actorID, settlementID int64) (domain.Settlement, error) {
	tx, err := s.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return domain.Settlement{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var value domain.Settlement
	err = tx.QueryRow(ctx, `SELECT id,group_id,sender_user_id,receiver_user_id,amount_minor,currency,status,created_by,version,created_at,confirmed_at FROM settlements WHERE id=$1 FOR UPDATE`, settlementID).Scan(&value.ID, &value.GroupID, &value.SenderUserID, &value.ReceiverUserID, &value.AmountMinor, &value.Currency, &value.Status, &value.CreatedBy, &value.Version, &value.CreatedAt, &value.ConfirmedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Settlement{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Settlement{}, err
	}
	if value.ReceiverUserID != actorID {
		return domain.Settlement{}, domain.ErrForbidden
	}
	if value.Status == domain.SettlementConfirmed {
		if err = tx.Commit(ctx); err != nil {
			return domain.Settlement{}, err
		}
		return value, nil
	}
	if value.Status != domain.SettlementPending {
		return domain.Settlement{}, domain.ErrInvalidState
	}
	err = tx.QueryRow(ctx, `UPDATE settlements SET status='confirmed',version=version+1,confirmed_at=NOW() WHERE id=$1 RETURNING status,version,confirmed_at`, settlementID).Scan(&value.Status, &value.Version, &value.ConfirmedAt)
	if err != nil {
		return domain.Settlement{}, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO audit_log(group_id,actor_user_id,action,entity_type,entity_id,entity_version,metadata) VALUES($1,$2,'settlement.confirmed','settlement',$3,$4,'{}')`, value.GroupID, actorID, value.ID, value.Version); err != nil {
		return domain.Settlement{}, err
	}
	if err = tx.Commit(ctx); err != nil {
		return domain.Settlement{}, err
	}
	return value, nil
}
