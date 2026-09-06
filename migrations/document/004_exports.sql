CREATE TABLE document_exports (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    format TEXT NOT NULL CHECK (format IN ('CSV', 'PDF', 'XLSX')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'processing', 'ready', 'failed')),
    object_key TEXT NULL UNIQUE,
    filename TEXT NOT NULL,
    error_code TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    finished_at TIMESTAMPTZ NULL
);

CREATE INDEX document_exports_actor_group_idx
    ON document_exports (actor_user_id, group_id);
CREATE INDEX document_exports_status_idx ON document_exports (status);
