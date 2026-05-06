## ADDED Requirements

### Requirement: Inbound Message Endpoint

The backend SHALL expose `POST /api/message` accepting a JSON payload from
n8n that contains the user's phone number and the raw text of the
WhatsApp message. The endpoint SHALL be the only entry point through
which transactional intents enter the system.

#### Scenario: Well-formed payload is accepted

- **WHEN** n8n sends `POST /api/message` with body
  `{"phone": "+62...", "message": "makan siang 45rb", "received_at": "<RFC3339>"}`
- **THEN** the backend responds with HTTP 200 and a JSON reply body
- **AND** no fields other than `phone`, `message`, `received_at` are required

#### Scenario: Missing required field is rejected

- **WHEN** the payload omits `phone` or `message`
- **THEN** the backend responds with HTTP 400 and an error body
  `{"error": "<field> is required"}`
- **AND** nothing is written to the database

#### Scenario: Unknown user phone

- **WHEN** the `phone` value does not match any registered user
- **THEN** the backend responds with HTTP 404 and an error body
  `{"error": "user not registered"}`

### Requirement: LLM Parse Output Schema

The LLM understanding layer SHALL return a `ParseResult` whose JSON
shape is fixed and validated before use. Any deviation SHALL be treated
as a parse failure.

#### Scenario: Conformant parse result is consumed

- **WHEN** the LLM returns
  `{"intent": "expense", "amount": 45000, "category": "Makan & Minum", "note": "makan siang", "confidence": 0.95}`
- **THEN** the usecase proceeds with the value
- **AND** `intent` is one of `expense`, `income`, `balance`, `report`,
  `delete_last`, `set_goal`, `set_budget`, `check_goal`, `unknown`
- **AND** `amount` is a non-negative integer in Rupiah
- **AND** `confidence` is a float between `0.0` and `1.0`

#### Scenario: Non-conformant parse result is rejected

- **WHEN** the LLM returns a payload that is not valid JSON, or omits a
  required field, or returns an `intent` outside the allowed set
- **THEN** the usecase logs the raw LLM output for audit
- **AND** treats the parse as `intent = unknown` with `confidence = 0`

### Requirement: Confidence Gate

A parse result with `confidence < 0.75` SHALL NOT be persisted. The
endpoint SHALL instead return a clarification reply for n8n to forward
to the user.

#### Scenario: Low-confidence parse triggers clarification

- **WHEN** the LLM returns `confidence = 0.60` for an expense
- **THEN** the backend responds with HTTP 200 and a body containing
  `{"reply_type": "clarify", "reply_text": "<question>", "persisted": false}`
- **AND** no row is written to the `transactions` table

#### Scenario: High-confidence parse is persisted

- **WHEN** the LLM returns `confidence >= 0.75` for an expense
- **THEN** a row is written to `transactions`
- **AND** the response body has `"persisted": true`

#### Scenario: Unknown intent is always clarified

- **WHEN** `intent = unknown` regardless of confidence
- **THEN** the response is a clarification reply
- **AND** nothing is persisted

### Requirement: Raw Message Audit

Every persisted transaction SHALL store the original WhatsApp text
verbatim in the `raw_message` column. The text SHALL NOT be normalized,
trimmed beyond surrounding whitespace, or truncated.

#### Scenario: Raw text is preserved

- **WHEN** the user sends `"  makan siang sama TEMEN 45rb!! 🍜  "`
- **AND** the parse succeeds with high confidence
- **THEN** `transactions.raw_message` is exactly
  `"makan siang sama TEMEN 45rb!! 🍜"` (only outer whitespace stripped)

### Requirement: Post-Transaction Reply Context

When a transaction is persisted, the response body SHALL include the
data needed to render a contextual WhatsApp reply without further
backend round-trips: current balance, remaining budget for the
transaction's category (if a budget exists), and progress on every
active savings goal.

#### Scenario: Expense with active budget and goal

- **GIVEN** the user has an active budget for `Makan & Minum` of
  Rp 500.000 and an active savings goal `Laptop` at 32% progress
- **WHEN** an expense of Rp 45.000 in `Makan & Minum` is persisted
- **THEN** the response body contains
  ```
  {
    "reply_type": "confirm",
    "persisted": true,
    "transaction": {"id": "<uuid>", "amount": 45000, "category": "Makan & Minum", "type": "expense"},
    "context": {
      "balance": <int64>,
      "category_budget": {"category": "Makan & Minum", "spent": <int64>, "limit": 500000, "remaining": <int64>},
      "savings_goals": [{"name": "Laptop", "saved": <int64>, "target": <int64>, "progress_pct": 32}]
    },
    "reply_text": "<LLM-composed natural-language summary>"
  }
  ```

#### Scenario: Expense without a budget for that category

- **GIVEN** the user has no budget set for `Hiburan`
- **WHEN** an expense in `Hiburan` is persisted
- **THEN** `context.category_budget` is `null`
- **AND** the rest of the context fields are still populated

#### Scenario: User has no active savings goals

- **WHEN** the user has no active goals
- **THEN** `context.savings_goals` is an empty array `[]`
- **AND** the field is still present (not omitted)

### Requirement: Idempotency by Received Timestamp

The backend SHALL deduplicate inbound messages on `(user_id, received_at)`
so that a retried webhook delivery MUST NOT create a second transaction
row.

#### Scenario: Duplicate webhook delivery

- **WHEN** n8n posts the same `(phone, received_at, message)` twice
- **THEN** only one row exists in `transactions`
- **AND** the second response returns the same `transaction.id` as the first
