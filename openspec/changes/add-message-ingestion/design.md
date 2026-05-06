## Context

The backend already has a partial implementation of message ingestion
under `internal/handler/message_handler.go`,
`internal/usecase/message_usecase.go`, and `internal/platform/llm/`.
Code exists but no contract is documented, so:

- n8n authors edit the WhatsApp webhook payload shape ad-hoc.
- The LLM prompt drifts when categories or intents are added, with no
  test that the JSON schema is still respected.
- The confidence threshold is sometimes applied at the handler, sometimes
  at the usecase, sometimes not at all.

This design pins the contract so all three layers (n8n workflow, Go
backend, Claude prompt) target the same shape.

## Goals / Non-Goals

**Goals:**
- One canonical contract for `POST /api/message`.
- LLM parse schema is validated before any business logic runs.
- Confidence gate is enforced in exactly one place (the usecase).
- Post-transaction reply payload is rich enough that n8n never needs a
  second backend call to render the WA message.
- Idempotency keyed on `(phone, received_at)` so retries are safe.

**Non-Goals:**
- Implementing the `set_goal`, `set_budget`, `check_goal`, `report`
  intents — they are listed in the parse schema but their handlers are
  separate change proposals.
- Composing the natural-language `reply_text`. That is owned by the
  `reminder-composition` capability (separate proposal). For ingestion,
  `reply_text` is a free-form string the usecase asks the composer to
  produce; the spec only requires the field to be present.
- Multi-user / family accounts.
- Rate limiting, abuse handling.

## Decisions

### Decision 1: Confidence gate lives in the usecase, not the handler

The handler is HTTP plumbing. The gate is business logic (it decides
whether to persist) and depends on the parsed result, so it belongs in
`message_usecase.go`. Putting it in the handler would force handler tests
to know about the LLM response shape.

### Decision 2: `intent = unknown` is always clarified, regardless of confidence

If the LLM is highly confident the intent is `unknown`, we still cannot
act on it. Treating it as a clarification request avoids a corner case
where confident-but-useless parses slip through.

### Decision 3: Idempotency via `(phone, received_at)` unique index, not message hash

A user can legitimately send the same message twice ("kopi 20rb" two
days apart). Hashing the body would falsely deduplicate those. The
`received_at` field comes from n8n's webhook payload and is stable
across retries within a single delivery, but unique across separate
sends. The migration adds:
`CREATE UNIQUE INDEX transactions_dedupe ON transactions (user_id, received_at)`.

### Decision 4: Parse result validation lives in `pkg/understanding`, not the LLM client

The LLM client (`platform/llm/client.go`) returns raw bytes. Validation
(JSON shape, intent enum, amount/confidence ranges) is performed in a
helper exposed by `pkg/understanding` so it can be unit-tested
independently of network calls. A malformed parse is downgraded to
`{intent: unknown, confidence: 0}` and the raw output is logged.

### Decision 5: `context` fields use `null` for "not applicable" and `[]` for "empty list"

Avoids ambiguity for n8n's templating: a missing budget is `null`
(distinguishable from a budget with `remaining = 0`); zero active goals
is `[]` (always present so the template can iterate safely).

### Decision 6: Reply payload always carries `reply_text`

Even though composing it is owned by another capability, including the
field in the ingestion contract means n8n always reads the same field
regardless of which capability produced the message. The composer is
called from the usecase as a dependency-inverted port
(`MessageComposer` interface in `domain/`).

## Risks / Trade-offs

- **LLM schema drift.** If Claude's structured-output behavior changes
  across model versions, validation will start downgrading parses to
  `unknown`. Mitigation: `parser_test.go` runs a battery of canonical
  inputs against the live model in CI nightly (not on every PR).
- **Confidence threshold tuning.** `0.75` is a guess. We accept it for
  the MVP and revisit once we have real production parses to score.
- **Idempotency window.** The unique index assumes `received_at` is
  preserved by n8n on retries. If n8n regenerates the timestamp, we get
  duplicates. Mitigation: spec the n8n workflow to forward
  `received_at` from the WhatsApp Business API event verbatim, not
  `Date.now()`.
- **Reply payload coupling.** The post-transaction context shape is now
  part of the public contract. Adding fields is safe (n8n ignores them);
  removing or renaming requires a spec change.
