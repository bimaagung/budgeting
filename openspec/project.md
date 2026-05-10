# Konteks Project untuk OpenSpec

File ini memberi pengguna OpenSpec (dan AI assistant) pointer kanonik ke konvensi project.

## Bahasa Dokumen

**Semua artifact OpenSpec di project ini ditulis dalam Bahasa Indonesia** — termasuk
`proposal.md`, `design.md`, spec deltas (`specs/**/*.md`), dan `tasks.md`.

Pengecualian (tetap dalam Bahasa Inggris):
- Identifier kode: nama function, struct, package, file path.
- Nilai enum domain: `expense`, `income`, `balance`, `report`, `delete_last`,
  `set_goal`, `set_budget`, `check_goal`, `unknown`.
- Header struktural OpenSpec yang sudah baku: `## ADDED Requirements`,
  `### Requirement:`, `#### Scenario:`, `WHEN`, `THEN`, `AND`, `GIVEN`.
- Istilah teknis tanpa padanan natural: `Clean Architecture`, `confidence gate`,
  `webhook`, `cron`, dll.

## Sumber Otoritatif

Overview project lengkap, arsitektur, tech stack, dan konvensi ada di
[`/CLAUDE.md`](../CLAUDE.md). File tersebut adalah sumber kebenaran.

## Ringkasan Singkat

- **Domain:** Budgeting personal via WhatsApp; LLM parsing pesan bebas menjadi transaksi.
- **Backend:** Golang, Clean Architecture (4 layer: `domain` → `usecase` → `handler` + `platform`).
- **Mobile:** Flutter (Android), dashboard read-only.
- **Automation:** n8n (forward pesan WA → backend; cron memicu reminder).
- **LLM:** Claude API untuk understanding dan komposisi reminder.

## Pemetaan Spec ke Kode

| Kapabilitas OpenSpec        | Lokasi Backend                             |
|-----------------------------|--------------------------------------------|
| `message-ingestion`         | `internal/handler/message_handler.go`, `internal/usecase/message_usecase.go`, `internal/platform/llm/` |
| `reminder-composition`      | `internal/handler/reminder_handler.go`, `internal/usecase/reminder_usecase.go`, `internal/platform/llm/composer.go` |
| `savings-goal`              | `internal/domain/savings_goal.go`, `internal/usecase/goal_usecase.go` |
| `budget-target`             | `internal/domain/budget_target.go`, `internal/usecase/goal_usecase.go` |

## Aturan Arsitektur (wajib dipatuhi setiap change)

Arah dependency: `handler`/`platform` → `usecase` → `domain`. Layer `domain`
tidak boleh import package internal lain. Implementasi platform
(Postgres, LLM client) berada di balik interface yang dideklarasikan di `domain`.

## Confidence Gate

Semua spec yang menyentuh LLM parsing wajib menghormati confidence threshold
(`< 0.75` memicu reply klarifikasi via WhatsApp sebelum melakukan persistensi).
