CREATE TABLE budget_targets (
    id       UUID        PRIMARY KEY DEFAULT uuid_generate_v4(),
    user_id  UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category VARCHAR(50) NOT NULL,
    amount   BIGINT      NOT NULL CHECK (amount > 0),
    month    SMALLINT    NOT NULL CHECK (month BETWEEN 1 AND 12),
    year     SMALLINT    NOT NULL,
    UNIQUE (user_id, category, month, year)
);

CREATE INDEX idx_budget_targets_user_month ON budget_targets(user_id, year, month);
