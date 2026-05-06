CREATE TABLE transactions (
    id          UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id     UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type        VARCHAR(10) NOT NULL CHECK (type IN ('income', 'expense')),
    amount      BIGINT      NOT NULL CHECK (amount > 0),
    category    VARCHAR(50) NOT NULL,
    note        TEXT        NOT NULL DEFAULT '',
    date        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    raw_message TEXT        NOT NULL DEFAULT '',
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_transactions_user_id ON transactions(user_id);
CREATE INDEX idx_transactions_date    ON transactions(date);
