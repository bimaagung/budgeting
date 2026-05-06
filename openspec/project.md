# Project Context for OpenSpec

This file gives OpenSpec users (and AI assistants) the canonical pointer to project conventions.

## Authoritative Source

The full project overview, architecture, tech stack, and conventions live in
[`/CLAUDE.md`](../CLAUDE.md). Treat that file as the source of truth.

## Quick Summary

- **Domain:** Personal budgeting via WhatsApp; LLM parses free-text into transactions.
- **Backend:** Golang, Clean Architecture (4 layers: `domain` → `usecase` → `handler` + `platform`).
- **Mobile:** Flutter (Android), read-only dashboard.
- **Automation:** n8n (forwards WA → backend; cron triggers reminders).
- **LLM:** Claude API for understanding and reminder composition.

## Where Specs Map to Code

| OpenSpec capability         | Backend location                           |
|-----------------------------|--------------------------------------------|
| `message-ingestion`         | `internal/handler/message_handler.go`, `internal/usecase/message_usecase.go`, `internal/platform/llm/` |
| `reminder-composition`      | `internal/handler/reminder_handler.go`, `internal/usecase/reminder_usecase.go`, `internal/platform/llm/composer.go` |
| `savings-goal`              | `internal/domain/savings_goal.go`, `internal/usecase/goal_usecase.go` |
| `budget-target`             | `internal/domain/budget_target.go`, `internal/usecase/goal_usecase.go` |

## Architecture Rule (must hold for every change)

Dependency direction: `handler`/`platform` → `usecase` → `domain`. The `domain`
layer must not import any other internal package. Platform implementations
(Postgres, LLM client) live behind interfaces declared in `domain`.

## Confidence Gate

Any spec touching LLM parsing must respect the confidence threshold (`< 0.75`
triggers a clarification reply via WhatsApp before persisting).
