# Rekomendasi Model Claude

## Coding Assistant (Claude Code)

| Fase | Model | Alasan |
|---|---|---|
| Perencanaan (proposal, design, spec) | `claude-opus-4-7` | Perlu reasoning mendalam untuk arsitektur, trade-off, edge case |
| Implementasi (tulis kode dari spec) | `claude-sonnet-4-6` | Spec sudah memutuskan desain; Sonnet lebih hemat & cepat untuk eksekusi |
| Subtask rutin (lint, rename, refactor mekanis) | `claude-haiku-4-5-20251001` | Paling murah, cukup untuk tugas mekanis |
| Review sebelum archive (`/review`, `/security-review`) | `claude-opus-4-7` | Pemeriksaan cermat, alasan sama dengan perencanaan |

**Default praktis (satu model):** `claude-sonnet-4-6` — cukup untuk semua fase; gunakan Opus hanya untuk proposal arsitektur kritis.

## LLM Layer di Aplikasi (Claude API — parsing WA & reminder)

| Kegunaan | Model | Alasan |
|---|---|---|
| Parsing pesan bebas → `ParseResult` JSON | `claude-haiku-4-5-20251001` | Hot path volume tinggi; structured output + system prompt ketat sudah cukup |
| Menyusun teks reminder (`MessageComposer`) | `claude-sonnet-4-6` | Butuh phrasing natural & motivatif dalam Bahasa Indonesia |
| Fallback low-confidence (retry sebelum klarifikasi ke user) | `claude-sonnet-4-6` | Retry 1x dengan Sonnet kalau Haiku return confidence < 0.75 |

> Confidence threshold aplikasi = 0.75 (per spec `add-message-ingestion`).
> Strategi dua-tier (Haiku → Sonnet retry) mengurangi clarification round-trip WA.
