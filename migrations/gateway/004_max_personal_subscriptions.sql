CREATE TABLE IF NOT EXISTS gateway_max_personal_subscriptions (
    max_user_id BIGINT PRIMARY KEY,
    chat_id BIGINT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'disabled')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS gateway_max_personal_subscriptions_active_idx
    ON gateway_max_personal_subscriptions (max_user_id)
    WHERE status = 'active';
