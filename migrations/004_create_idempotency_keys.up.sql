CREATE TABLE idempotency_keys (
    key           TEXT PRIMARY KEY,
    request_hash  TEXT NOT NULL,
    status_code   INTEGER,
    response_body JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
