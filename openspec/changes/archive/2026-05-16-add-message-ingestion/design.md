## Context

Backend sudah memiliki implementasi parsial dari message ingestion di
`internal/handler/message_handler.go`,
`internal/usecase/message_usecase.go`, dan `internal/platform/llm/`.
Kode sudah ada tetapi kontraknya belum terdokumentasi, sehingga:

- Penulis workflow n8n mengedit bentuk payload webhook WhatsApp secara
  ad-hoc.
- Prompt LLM bergeser ketika kategori atau intent ditambahkan, tanpa
  ada test bahwa skema JSON masih dipatuhi.
- Confidence threshold kadang diterapkan di handler, kadang di usecase,
  kadang tidak sama sekali.
- Response endpoint berbeda bentuk untuk tiap intent (kadang object,
  kadang string), sehingga n8n harus memiliki cabang per intent.
- User otomatis dibuat saat phone tidak ditemukan, yang membuat siapa
  pun yang mengetahui nomor backend bisa menjadi user.

Design ini menetapkan kontrak agar ketiga layer (workflow n8n, backend
Go, prompt Claude) menargetkan bentuk yang sama, dan agar response
endpoint seragam untuk semua intent.

## Goals / Non-Goals

**Goals:**
- Satu kontrak kanonik untuk `POST /api/message`.
- Skema parse LLM divalidasi sebelum business logic apa pun berjalan.
- Confidence gate ditegakkan di tepat satu tempat (di usecase).
- Response endpoint memiliki bentuk tagged union yang seragam untuk
  semua 9 intent (`expense`, `income`, `balance`, `delete_last`,
  `set_goal`, `set_budget`, `check_goal`, `report`, `unknown`).
- Payload `reply_type = confirm` cukup kaya sehingga n8n tidak perlu
  panggilan backend kedua untuk merender pesan WA.
- Idempotency dengan kunci `(user_id, received_at)` agar retry aman.
- Endpoint selalu mengembalikan reply yang bisa diteruskan ke user;
  kegagalan internal tidak meninggalkan user dalam kebisuan.

**Non-Goals:**
- Komposisi `reply_text` natural-language. Itu dimiliki oleh kapabilitas
  `reminder-composition` (proposal terpisah). Untuk ingestion,
  `reply_text` adalah string bebas yang diminta usecase ke composer
  hanya untuk `reply_type = confirm`; spec hanya mensyaratkan field
  tersebut hadir untuk semua reply_type.
- Implementasi penuh lifecycle untuk intent `set_goal`, `set_budget`,
  `check_goal`, `report` — intent tersebut tercantum di skema parse dan
  endpoint melakukan dispatch ke handler-nya, namun spek detail per
  lifecycle merupakan proposal change terpisah.
- Onboarding user. Endpoint hanya melakukan lookup; pendaftaran user
  baru dilakukan via channel lain (Flutter, seed manual).
- Akun multi-user / keluarga.
- Rate limiting, penanganan abuse.
- Authentication/authorization request dari n8n (diasumsikan trusted
  network atau shared secret di reverse proxy).

## Decisions

### Decision 1: Confidence gate berada di usecase, bukan handler

Handler adalah HTTP plumbing. Gate adalah business logic (memutuskan
apakah persist atau tidak) dan bergantung pada hasil parse, jadi
tempatnya di `message_usecase.go`. Menempatkannya di handler akan
memaksa test handler untuk mengetahui bentuk respons LLM.

### Decision 2: `intent = unknown` selalu diklarifikasi, terlepas dari confidence

Jika LLM sangat yakin intent-nya `unknown`, kita tetap tidak bisa
bertindak. Memperlakukannya sebagai permintaan klarifikasi mencegah
corner case di mana parse confident-but-useless lolos.

### Decision 3: Idempotency via unique index `(user_id, received_at)`, bukan hash pesan

User bisa secara sah mengirim pesan yang sama dua kali ("kopi 20rb" pada
dua hari berbeda). Hashing body akan men-dedup secara salah. Field
`received_at` berasal dari payload webhook n8n dan stabil lintas retry
dalam satu delivery, namun unik antar pengiriman terpisah. Migration
menambahkan:
`CREATE UNIQUE INDEX transactions_dedupe ON transactions (user_id, received_at)`.

### Decision 4: Validasi hasil parse berada di `internal/platform/llm/parser.go`

LLM client mengembalikan raw bytes dari Claude API. Validasi (bentuk
JSON, enum intent, range amount/confidence) dilakukan di helper
`Validate(raw []byte) (ParseResult, error)` yang berada di
`internal/platform/llm/parser.go` sebagai pure function (tidak
melakukan I/O). Helper ini dipasang di call site composer; saat
validasi gagal, raw output di-log dan parse diturunkan menjadi
`{intent: unknown, confidence: 0}`.

Lokasi `internal/platform/llm/` (bukan `pkg/understanding/`) dipilih
karena fungsi ini hanya punya satu caller (composer LLM yang sama).
Promosi ke `pkg/` dilakukan saat ada caller kedua. CLAUDE.md akan
disinkronkan dengan keputusan ini.

### Decision 5: Field `context` dan `data` menggunakan `null` untuk "tidak berlaku" dan `[]` untuk "list kosong"

Menghindari ambiguitas untuk templating n8n: budget yang tidak ada
bernilai `null` (bisa dibedakan dari budget dengan `remaining = 0`); list
goal aktif kosong bernilai `[]` (selalu hadir agar template bisa
iterasi dengan aman).

### Decision 6: Setiap response selalu membawa `reply_text` non-kosong

Meski penyusunannya bisa dimiliki kapabilitas lain, menyertakan field ini
di kontrak ingestion berarti n8n selalu membaca field yang sama
terlepas dari `reply_type` mana yang dihasilkan. Sumber `reply_text`
ditentukan oleh Decision 9 (hybrid).

### Decision 7: User registration policy — reject 404, no auto-create

Endpoint melakukan lookup ketat via `FindByPhone`. Jika user tidak
ditemukan, return HTTP 404 `{"error": "user not registered"}`.
Pendaftaran user dilakukan via channel lain (out of scope change ini).

Alternatif "auto-create on first message" ditolak karena membuka surface
abuse: siapa pun yang menemukan nomor backend bisa membuat row user
liar. Onboarding eksplisit lebih aman, walaupun butuh channel
pendaftaran terpisah (Flutter, seed manual untuk MVP single-user).

Konsekuensi: kode existing `getOrCreateUser` di
`message_usecase.go:128–137` di-refactor menjadi panggilan
`userRepo.FindByPhone` saja; usecase return error "user not registered"
yang di-mapping handler ke HTTP 404.

### Decision 8: Tagged union response envelope

Response endpoint memiliki bentuk:

```json
{
  "reply_type": "confirm" | "clarify" | "info" | "error",
  "persisted": <bool>,
  "reply_text": "<string non-kosong>",
  "transaction": {...},   // hanya saat reply_type = confirm
  "context": {...},       // hanya saat reply_type = confirm
  "data": {...}           // hanya saat reply_type = info
}
```

Per-`reply_type`:

| reply_type | persisted | Field tambahan | Trigger |
|---|---|---|---|
| `confirm` | `true` | `transaction`, `context` | expense/income parse confidence ≥ 0.75 → persist sukses |
| `clarify` | `false` | — | confidence < 0.75, intent unknown, set_goal/set_budget tanpa amount valid, report (sementara) |
| `info` | `false` | `data` | balance, delete_last, check_goal sukses |
| `error` | `false` | — | downstream gagal setelah parse sukses |

Alternatif "flat string `reply_text`" ditolak karena Flutter dan debug
butuh structured context. Alternatif "per-intent shape heterogen"
ditolak karena n8n harus switch logic per intent, dan setiap intent
baru melahirkan breaking change di workflow.

### Decision 9: Hybrid `reply_text` source — LLM hanya untuk `confirm`

`reply_text` di-generate berbeda per `reply_type`:

- `confirm` → composer LLM (susun pesan kontekstual saldo + budget +
  goal). Composer port: `domain.MessageComposer` (interface yang sudah
  ada di codebase, dipakai juga oleh kapabilitas
  reminder-composition nanti).
- `clarify` → template deterministik di `internal/usecase` per
  sub-skenario (low-confidence, unknown, missing amount, report).
- `info` → template deterministik per intent (balance, delete_last,
  check_goal) dengan substitusi dari `data`.
- `error` → template deterministik tetap: `"Maaf, ada gangguan
  sebentar. Coba lagi sebentar lagi 🙏"`.

Alasan tidak semua via LLM: (a) cost — `info`/`clarify` tidak butuh
kreativitas; (b) latency — query `balance` harus <1s sehingga panggilan
LLM extra tidak bisa diterima; (c) reliability — `error` path tidak
boleh tergantung LLM yang justru sedang down.

### Decision 10: Internal failure post-parse → HTTP 200 + `reply_type = error`

Setelah parse LLM sukses, error apa pun di tahap berikutnya (save DB,
panggilan composer, port lain) WAJIB dikoreksi menjadi
`reply_type = error` dengan HTTP 200. Detail error dilog. HTTP 4xx
tetap dipakai HANYA untuk:
- 400 — request body malformed (missing field, format salah).
- 404 — user tidak terdaftar (sebelum parse dijalankan).

Alasan: n8n adalah dumb forwarder. Jika backend kasih 5xx, n8n tidak
punya `reply_text` yang bisa diteruskan ke user, sehingga UX hancur.
Spec menjamin: parse sukses → user selalu dapat reply yang aman
diteruskan ke WA. Observability tidak hilang karena log error masih
intact.

Pengecualian: composer LLM gagal saat menyusun `reply_text` untuk
`reply_type = confirm` — transaksi sudah dipersist, jadi response tetap
`reply_type = confirm` (bukan `error`) dengan `reply_text` template
fallback "Tercatat ✅" + ringkasan minimal. Membalik menjadi `error` akan
menyesatkan: data sudah masuk DB.

### Decision 11: Phone normalization di backend → E.164

Handler menormalisasi `phone` ke E.164 (`+62...`) sebelum lookup user
dan sebelum dedup key dibentuk. Format yang diterima dari n8n:

- `+62812...` (sudah E.164) → as-is
- `62812...` → tambah `+`
- `0812...` → ganti `0` dengan `+62` (asumsi default Indonesia untuk
  MVP)
- format lain → HTTP 400 `{"error": "invalid phone format"}`

Alasan: n8n bisa berubah perilaku saat upgrade workflow atau saat WA
Business API berganti format. Normalization di backend membuat sistem
robust terhadap variasi tanpa breaking change ke n8n. Untuk multi-region
nanti, default Indonesia akan diparametrisasi.

### Decision 12: Idempotent retry → fresh context + re-compose `reply_text`

Saat dedup match `(user_id, received_at)` di repo `Save`,
unique-violation diturunkan menjadi return existing row (tidak error).
Usecase melanjutkan jalur normal: build context dari state terbaru +
panggil composer untuk `reply_text` baru.

Konsekuensi: response kedua untuk webhook retry punya `transaction.id`
sama dengan yang pertama, tapi `context.balance` (dst.) dan
`reply_text` bisa berbeda karena state user mungkin sudah berubah
(transaksi lain di antara dua delivery).

Alternatif "cache response pertama dan replay byte-for-byte" ditolak
karena: (a) butuh storage tambahan (Redis atau tabel), (b) bisa kasih
user data stale (saldo dari beberapa menit lalu), (c) cost LLM extra
per-retry dianggap dapat ditolerir karena retry rare di kondisi
operasional normal.

## Risks / Trade-offs

- **Drift skema LLM.** Jika perilaku structured-output Claude berubah
  antar versi model, validasi akan mulai menurunkan parse menjadi
  `unknown`. Mitigasi: `parser_test.go` menjalankan rangkaian input
  kanonik terhadap model live di CI nightly (bukan setiap PR).
- **Tuning confidence threshold.** `0.75` adalah tebakan. Kita terima
  untuk MVP dan tinjau ulang setelah punya parse produksi nyata untuk
  di-score.
- **Window idempotency.** Unique index mengasumsikan `received_at`
  dipertahankan oleh n8n saat retry. Jika n8n meregenerasi timestamp,
  duplikat akan muncul. Mitigasi: spesifikasikan di workflow n8n agar
  meneruskan `received_at` dari event WhatsApp Business API apa adanya,
  bukan `Date.now()`.
- **Coupling response envelope.** Bentuk envelope kini menjadi bagian
  kontrak publik. Menambah field aman (n8n mengabaikannya); menghapus
  atau mengubah nama membutuhkan spec change.
- **`reply_type = error` menyembunyikan 5xx.** Monitoring perlu
  diperhatikan: dashboard tidak boleh hanya mengandalkan HTTP status,
  tetapi juga rate `reply_type = error` di response log.
- **Onboarding via channel lain.** Reject 404 berarti user pertama
  tidak bisa mendaftar via WA. Mitigasi MVP: seed user owner secara
  manual di DB; channel onboarding eksplisit dispec terpisah.
- **Phone format Indonesia-only.** Normalizer hard-code default Indonesia
  (`0` → `+62`). Multi-region butuh refactor. Acceptable untuk MVP
  single-user.
- **Composer fallback saat confirm.** Decision 10 menetapkan jika
  composer gagal saat susun `reply_text` confirm, gunakan template
  minimal. Trade-off: user dapat pesan kurang motivatif, tapi data
  sudah aman di DB dan user tahu transaksi tercatat.
