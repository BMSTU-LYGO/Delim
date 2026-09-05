CREATE TABLE document_receipts (
    id BIGSERIAL PRIMARY KEY,
    actor_user_id BIGINT NOT NULL,
    group_id BIGINT NOT NULL,
    filename TEXT NOT NULL,
    content_type TEXT NOT NULL,
    size_bytes BIGINT NOT NULL,
    object_key TEXT NOT NULL UNIQUE,
    status TEXT NOT NULL CHECK (status IN (
        'uploaded', 'queued', 'processing', 'ready', 'failed', 'deleted'
    )),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMPTZ NULL
);

CREATE INDEX document_receipts_actor_group_idx
    ON document_receipts (actor_user_id, group_id);
CREATE INDEX document_receipts_status_idx
    ON document_receipts (status);

CREATE TABLE document_jobs (
    id BIGSERIAL PRIMARY KEY,
    receipt_id BIGINT NOT NULL REFERENCES document_receipts (id),
    type TEXT NOT NULL CHECK (type IN ('OCR')),
    status TEXT NOT NULL CHECK (status IN (
        'pending', 'processing', 'completed', 'failed'
    )),
    attempts INT NOT NULL DEFAULT 0,
    error_code TEXT NULL,
    error_message TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    started_at TIMESTAMPTZ NULL,
    finished_at TIMESTAMPTZ NULL
);

CREATE INDEX document_jobs_receipt_idx ON document_jobs (receipt_id);
CREATE INDEX document_jobs_status_idx ON document_jobs (status);
