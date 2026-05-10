-- Add received_at column (event time from WhatsApp Business API, forwarded by n8n).
-- Default NOW() so existing rows don't violate NOT NULL; new rows always pass an explicit value.
ALTER TABLE transactions
    ADD COLUMN received_at TIMESTAMPTZ NOT NULL DEFAULT NOW();

-- Idempotency: same (user, received_at) tuple cannot create two rows.
-- This lets repo treat unique-violation as "already persisted" and return the existing row.
CREATE UNIQUE INDEX transactions_dedupe ON transactions (user_id, received_at);
