CREATE TABLE IF NOT EXISTS gateway_max_chat_groups (
    chat_id BIGINT PRIMARY KEY,
    group_id BIGINT NOT NULL UNIQUE,
    bound_by_user_id BIGINT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('active', 'unbound')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

-- One active binding per group and per chat is guaranteed by the UNIQUE
-- group_id and PRIMARY KEY chat_id above. Historical unbound rows keep chat_id
-- as PK, so re-binding a previously-unbound chat updates the same row.
CREATE INDEX IF NOT EXISTS gateway_max_chat_groups_active_group_idx
    ON gateway_max_chat_groups (group_id)
    WHERE status = 'active';
