## 1. Pin the HTTP contract

- [ ] 1.1 Lock the `POST /api/message` request schema (`phone`, `message`, `received_at`) in `internal/handler/message_handler.go` and reject other shapes with HTTP 400.
- [ ] 1.2 Return HTTP 404 with `{"error": "user not registered"}` when `phone` does not resolve to a user.
- [ ] 1.3 Add a JSON schema fixture under `internal/handler/testdata/message_request.schema.json` and a handler-level test that loads it.

## 2. Validate LLM parse output

- [ ] 2.1 In `pkg/understanding/`, add a `Validate(raw []byte) (ParseResult, error)` helper that enforces the field set, the `intent` enum, `amount >= 0`, and `0 <= confidence <= 1`.
- [ ] 2.2 Wire `Validate` into the call site in `internal/platform/llm/`; on validation failure, log the raw output and return `ParseResult{Intent: "unknown", Confidence: 0}`.
- [ ] 2.3 Extend `internal/platform/llm/parser_test.go` with table-driven cases for each malformed shape (missing field, wrong type, out-of-range confidence, unknown intent).

## 3. Enforce the confidence gate in the usecase

- [ ] 3.1 In `internal/usecase/message_usecase.go`, route `confidence < 0.75` (or `intent = unknown`) to a clarification path that returns `reply_type = "clarify"` and does not persist.
- [ ] 3.2 Remove any duplicate confidence checks from the handler layer.
- [ ] 3.3 Add usecase-level tests covering: high-confidence persist, low-confidence clarify, unknown-intent clarify.

## 4. Persist `raw_message` verbatim

- [ ] 4.1 Trim only outer whitespace before passing the message to the parser; store the trimmed value in `transactions.raw_message`.
- [ ] 4.2 Add a usecase test that confirms emoji and internal whitespace survive the round-trip.

## 5. Build the post-transaction reply context

- [ ] 5.1 In `internal/usecase/message_usecase.go`, after persisting, fetch balance, the relevant `BudgetTarget` for the transaction's category and current month, and all active `SavingsGoal` rows.
- [ ] 5.2 Shape the response body to match the spec (use `null` for missing budget, `[]` for no goals).
- [ ] 5.3 Call the `MessageComposer` port to populate `reply_text`; on composer error, fall back to a deterministic minimal text and log the error.
- [ ] 5.4 Add a usecase test that asserts the response shape against a JSON schema fixture.

## 6. Idempotency

- [ ] 6.1 Add a migration `005_transactions_dedupe.sql` creating
  `CREATE UNIQUE INDEX transactions_dedupe ON transactions (user_id, received_at)`.
- [ ] 6.2 In `internal/platform/postgres/transaction_repo.go`, treat unique-violation on insert as "already persisted" and return the existing row's id.
- [ ] 6.3 Add a repo-level integration test that asserts double-insert returns the same id and only one row exists.

## 7. n8n contract

- [ ] 7.1 Update `n8n/workflows/wa-inbound.json` so the HTTP node forwards exactly `{phone, message, received_at}` and uses the WA event timestamp (not `Date.now()`).
- [ ] 7.2 Document the contract in a comment node inside the workflow JSON for future editors.

## 8. Verification

- [ ] 8.1 `cd backend && go test ./...` passes.
- [ ] 8.2 `openspec validate add-message-ingestion --strict` passes.
- [ ] 8.3 Manual smoke: send a WA message via the n8n test workflow and observe a row in `transactions` plus a context-rich reply payload in the response log.
