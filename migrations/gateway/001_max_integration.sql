CREATE TABLE IF NOT EXISTS gateway_max_chats (
    chat_id BIGINT PRIMARY KEY,
    is_channel BOOLEAN NOT NULL DEFAULT FALSE,
    status TEXT NOT NULL CHECK (status IN ('active', 'removed')),
    last_event_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS gateway_max_updates (
    event_key TEXT PRIMARY KEY,
    update_type TEXT NOT NULL,
    chat_id BIGINT,
    payload JSONB NOT NULL,
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'processed', 'failed')),
    attempts INTEGER NOT NULL DEFAULT 0,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_error TEXT,
    received_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    processed_at TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS gateway_max_updates_pending_idx
    ON gateway_max_updates (next_attempt_at, received_at)
    WHERE status = 'pending';
