CREATE TABLE document_ocr_results (
    id BIGSERIAL PRIMARY KEY,
    receipt_id BIGINT NOT NULL UNIQUE
        REFERENCES document_receipts (id) ON DELETE CASCADE,
    qr_raw TEXT NULL,
    merchant TEXT NULL,
    receipt_date TIMESTAMPTZ NULL,
    total_minor BIGINT NULL CHECK (total_minor >= 0),
    currency TEXT NULL,
    confidence DOUBLE PRECISION NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    raw_text TEXT NOT NULL,
    raw_lines JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE document_ocr_items (
    id BIGSERIAL PRIMARY KEY,
    result_id BIGINT NOT NULL
        REFERENCES document_ocr_results (id) ON DELETE CASCADE,
    position INT NOT NULL CHECK (position >= 0),
    name TEXT NOT NULL,
    quantity NUMERIC NULL CHECK (quantity > 0),
    unit_price_minor BIGINT NULL CHECK (unit_price_minor >= 0),
    amount_minor BIGINT NOT NULL CHECK (amount_minor >= 0),
    confidence DOUBLE PRECISION NOT NULL CHECK (confidence BETWEEN 0 AND 1),
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (result_id, position)
);
