## Why

The product depends on users typing free-text WhatsApp messages (e.g.
"makan siang sama temen 45rb") and having those messages turned into
structured transactions. Today this flow is undocumented as a contract:
n8n, the Go backend, and the LLM each make assumptions that drift apart.
Without a spec we cannot guarantee that:

- n8n forwards a stable payload shape (it currently does whatever the
  workflow author last edited).
- The LLM output schema stays consistent across model changes.
- Low-confidence parses always trigger a confirmation reply instead of
  silently writing wrong rows.

This change captures the message-ingestion capability as the first
OpenSpec spec so future changes (savings goals, budget alerts, etc.)
build on a documented foundation.

## What Changes

- Define the **`POST /api/message`** request/response contract so n8n
  has a fixed target and the backend can reject malformed payloads.
- Specify the **LLM parse output schema** (`intent`, `amount`, `category`,
  `note`, `confidence`) and the allowed `intent` values.
- Specify the **confidence gate** behavior: parses below `0.75` must not
  persist; instead they return a clarification reply for n8n to forward
  back to the user.
- Specify the **raw_message audit rule**: every accepted transaction
  stores the original text verbatim.
- Specify the **post-transaction reply contract**: every successful
  expense/income write returns a payload that includes balance + remaining
  category budget + active savings-goal progress, so n8n can render the
  reply without extra round-trips.

No code is changed in this proposal — only the spec is introduced. A
follow-up implementation change will reconcile current code with the
spec where they diverge.

## Capabilities

### New Capabilities
- `message-ingestion`: Contract for receiving a raw WhatsApp message,
  parsing it via the LLM, applying a confidence gate, persisting the
  resulting transaction (or requesting clarification), and returning a
  reply payload with balance + budget + goal context.

### Modified Capabilities
<!-- None — first spec in the project. -->

## Impact

- **Code paths covered:** `internal/handler/message_handler.go`,
  `internal/usecase/message_usecase.go`,
  `internal/platform/llm/prompt.go`,
  `internal/platform/llm/parser_test.go`.
- **External contracts pinned:** n8n `wa-inbound` workflow request body,
  Claude API structured output schema.
- **Database:** read-only against this change; uses the existing
  `transactions`, `budget_targets`, `savings_goals` tables.
- **Observability:** raw message audit field becomes part of the spec,
  not just a convention.
- **Out of scope (separate future changes):** reminder composition,
  savings-goal lifecycle, budget-target lifecycle, multi-user.
