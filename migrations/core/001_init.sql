CREATE TABLE users (
    id BIGSERIAL PRIMARY KEY,
    max_user_id BIGINT NOT NULL UNIQUE,
    first_name TEXT NOT NULL DEFAULT '',
    last_name TEXT NOT NULL DEFAULT '',
    username TEXT NOT NULL DEFAULT '',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE groups (
    id BIGSERIAL PRIMARY KEY,
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    owner_id BIGINT NOT NULL REFERENCES users(id),
    status TEXT NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'archived')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE group_members (
    group_id BIGINT NOT NULL REFERENCES groups(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    role TEXT NOT NULL CHECK (role IN ('owner', 'admin', 'member')),
    joined_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX group_members_user_group_idx ON group_members (user_id, group_id);

CREATE TABLE expenses (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id),
    payer_user_id BIGINT NOT NULL REFERENCES users(id),
    created_by BIGINT NOT NULL REFERENCES users(id),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    description TEXT NOT NULL DEFAULT '',
    expense_date TIMESTAMPTZ NOT NULL,
    split_type TEXT NOT NULL CHECK (split_type IN ('equal', 'fixed', 'shares', 'percentage', 'item')),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'cancelled')),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX expenses_group_list_idx ON expenses (group_id, id DESC);
CREATE INDEX expenses_group_status_idx ON expenses (group_id, status, currency);

CREATE TABLE expense_items (
    id BIGSERIAL PRIMARY KEY,
    expense_id BIGINT NOT NULL REFERENCES expenses(id),
    name TEXT NOT NULL CHECK (btrim(name) <> ''),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    position INTEGER NOT NULL CHECK (position >= 0),
    UNIQUE (expense_id, position)
);
CREATE INDEX expense_items_expense_idx ON expense_items (expense_id);

CREATE TABLE allocations (
    id BIGSERIAL PRIMARY KEY,
    expense_id BIGINT NOT NULL REFERENCES expenses(id),
    expense_item_id BIGINT REFERENCES expense_items(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0)
);
CREATE INDEX allocations_expense_idx ON allocations (expense_id);
CREATE INDEX allocations_user_expense_idx ON allocations (user_id, expense_id);
CREATE UNIQUE INDEX allocations_expense_user_no_item_uidx ON allocations (expense_id, user_id) WHERE expense_item_id IS NULL;
CREATE UNIQUE INDEX allocations_item_user_uidx ON allocations (expense_item_id, user_id) WHERE expense_item_id IS NOT NULL;

CREATE TABLE settlements (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id),
    sender_user_id BIGINT NOT NULL REFERENCES users(id),
    receiver_user_id BIGINT NOT NULL REFERENCES users(id),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'confirmed', 'cancelled')),
    created_by BIGINT NOT NULL REFERENCES users(id),
    version BIGINT NOT NULL DEFAULT 1 CHECK (version > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    confirmed_at TIMESTAMPTZ,
    CHECK (sender_user_id <> receiver_user_id),
    CHECK ((status = 'confirmed') = (confirmed_at IS NOT NULL))
);
CREATE INDEX settlements_group_list_idx ON settlements (group_id, id DESC);
CREATE INDEX settlements_group_status_idx ON settlements (group_id, status, currency);

CREATE TABLE adjustments (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT NOT NULL REFERENCES groups(id),
    expense_id BIGINT NOT NULL REFERENCES expenses(id),
    type TEXT NOT NULL CHECK (type IN ('refund', 'correction')),
    amount_minor BIGINT NOT NULL CHECK (amount_minor > 0),
    currency TEXT NOT NULL CHECK (currency ~ '^[A-Z]{3}$'),
    created_by BIGINT NOT NULL REFERENCES users(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX adjustments_expense_idx ON adjustments (expense_id, id);
CREATE INDEX adjustments_group_idx ON adjustments (group_id, currency);

CREATE TABLE adjustment_allocations (
    adjustment_id BIGINT NOT NULL REFERENCES adjustments(id),
    user_id BIGINT NOT NULL REFERENCES users(id),
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    PRIMARY KEY (adjustment_id, user_id)
);
CREATE INDEX adjustment_allocations_user_idx ON adjustment_allocations (user_id, adjustment_id);

CREATE TABLE audit_log (
    id BIGSERIAL PRIMARY KEY,
    group_id BIGINT REFERENCES groups(id),
    actor_user_id BIGINT NOT NULL REFERENCES users(id),
    action TEXT NOT NULL CHECK (btrim(action) <> ''),
    entity_type TEXT NOT NULL CHECK (btrim(entity_type) <> ''),
    entity_id BIGINT NOT NULL,
    entity_version BIGINT,
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX audit_log_group_created_idx ON audit_log (group_id, created_at DESC, id DESC);
CREATE INDEX audit_log_entity_idx ON audit_log (entity_type, entity_id, id);

CREATE FUNCTION forbid_audit_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'audit_log is append-only';
END;
$$;
CREATE TRIGGER audit_log_no_update_or_delete
BEFORE UPDATE OR DELETE ON audit_log
FOR EACH ROW EXECUTE FUNCTION forbid_audit_mutation();
