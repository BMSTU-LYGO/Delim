ALTER TABLE document_jobs
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

CREATE INDEX document_jobs_pending_claim_idx
    ON document_jobs (next_attempt_at, created_at)
    WHERE status = 'pending';
