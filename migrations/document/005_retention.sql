ALTER TABLE document_receipts ADD COLUMN original_purged_at TIMESTAMPTZ NULL;
ALTER TABLE document_exports ADD COLUMN object_purged_at TIMESTAMPTZ NULL;

CREATE INDEX document_receipts_retention_idx
    ON document_receipts (id)
    WHERE original_purged_at IS NULL;
CREATE INDEX document_exports_retention_idx
    ON document_exports (id)
    WHERE object_purged_at IS NULL;
