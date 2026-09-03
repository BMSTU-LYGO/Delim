package postgres

import (
	"context"
	"encoding/json"
	"github.com/jackc/pgx/v5"
)

type auditRecord struct {
	GroupID            *int64
	ActorID            int64
	Action, EntityType string
	EntityID           int64
	Version            *int64
	Metadata           map[string]string
}

func int64Pointer(value int64) *int64 { return &value }
func appendAudit(ctx context.Context, tx pgx.Tx, record auditRecord) error {
	if record.Metadata == nil {
		record.Metadata = map[string]string{}
	}
	metadata, err := json.Marshal(record.Metadata)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO audit_log(group_id,actor_user_id,action,entity_type,entity_id,entity_version,metadata) VALUES($1,$2,$3,$4,$5,$6,$7)`, record.GroupID, record.ActorID, record.Action, record.EntityType, record.EntityID, record.Version, metadata)
	return err
}
