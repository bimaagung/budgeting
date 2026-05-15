## ADDED Requirements

### Requirement: Inbound Message Endpoint

Backend WAJIB mengekspos `POST /api/message` yang menerima payload JSON
dari n8n yang berisi nomor telepon user, teks mentah pesan WhatsApp,
dan timestamp event. Endpoint ini WAJIB menjadi satu-satunya entry
point untuk pesan WhatsApp yang masuk ke sistem, terlepas dari intent.

#### Scenario: Well-formed payload is accepted

- **WHEN** n8n mengirim `POST /api/message` dengan body
  `{"phone": "+62...", "message": "makan siang 45rb", "received_at": "<RFC3339>"}`
- **THEN** backend merespons dengan HTTP 200 dan body JSON yang
  mengikuti response envelope tagged union (lihat requirement
  "Response Envelope")
- **AND** field selain `phone`, `message`, `received_at` di request
  diabaikan (forward-compatibility)

#### Scenario: Missing required field is rejected

- **WHEN** payload tidak menyertakan `phone`, `message`, atau
  `received_at`
- **THEN** backend merespons dengan HTTP 400 dan body error
  `{"error": "<field> is required"}`
- **AND** tidak ada apa pun yang ditulis ke database
- **AND** tidak ada panggilan ke LLM

#### Scenario: Invalid received_at format is rejected

- **WHEN** `received_at` bukan string RFC3339 valid
- **THEN** backend merespons dengan HTTP 400 dan body error
  `{"error": "received_at must be RFC3339"}`

#### Scenario: Unknown user phone

- **WHEN** nilai `phone` (setelah dinormalisasi ke E.164) tidak cocok
  dengan user yang terdaftar
- **THEN** backend merespons dengan HTTP 404 dan body error
  `{"error": "user not registered"}`
- **AND** endpoint TIDAK membuat user baru
- **AND** tidak ada panggilan ke LLM

### Requirement: Phone Normalization

Handler WAJIB menormalisasi nilai `phone` ke format E.164
(`+<country><number>`) sebelum lookup user, sebelum membentuk dedup
key, dan sebelum disimpan di tabel manapun. Nilai sebelum normalisasi
tidak boleh bocor ke layer di bawah handler.

#### Scenario: Variasi format Indonesia diterima

- **WHEN** n8n mengirim `phone` salah satu dari `+62812...`,
  `62812...`, atau `0812...`
- **THEN** handler menormalisasi ke `+62812...`
- **AND** lookup user dan dedup key menggunakan bentuk ter-normalisasi

#### Scenario: Format yang tidak dikenali ditolak

- **WHEN** `phone` tidak match salah satu pattern: `+62<digits>`,
  `62<digits>`, atau `0<digits>` (MVP single-region Indonesia)
- **THEN** backend merespons dengan HTTP 400 dan body error
  `{"error": "invalid phone format"}`
- **AND** phone E.164 dari country code lain (mis. `+1...`) ditolak di
  MVP; multi-region adalah change terpisah

### Requirement: LLM Parse Output Schema

Layer LLM understanding WAJIB mengembalikan `ParseResult` yang bentuk
JSON-nya sudah ditetapkan dan divalidasi sebelum digunakan. Setiap
penyimpangan WAJIB diperlakukan sebagai parse failure.

#### Scenario: Conformant parse result is consumed

- **WHEN** LLM mengembalikan
  `{"intent": "expense", "amount": 45000, "category": "Makan & Minum", "note": "makan siang", "confidence": 0.95}`
- **THEN** usecase melanjutkan dengan nilai tersebut
- **AND** `intent` adalah salah satu dari `expense`, `income`, `balance`,
  `report`, `delete_last`, `set_goal`, `set_budget`, `check_goal`,
  `unknown`
- **AND** `amount` adalah integer non-negatif dalam Rupiah
- **AND** `confidence` adalah float antara `0.0` dan `1.0`

#### Scenario: Non-conformant parse result is rejected

- **WHEN** LLM mengembalikan payload yang bukan JSON valid, atau
  melewatkan field wajib, atau mengembalikan `intent` di luar set yang
  diizinkan, atau `amount` negatif, atau `confidence` di luar range
- **THEN** validator mencatat raw output LLM untuk audit
- **AND** memperlakukan parse sebagai `intent = unknown` dengan
  `confidence = 0`

### Requirement: Confidence Gate

Hasil parse dengan `confidence < 0.75` WAJIB TIDAK dipersist. Endpoint
WAJIB justru mengembalikan response dengan `reply_type = clarify` yang
diteruskan n8n ke user.

#### Scenario: Low-confidence parse triggers clarification

- **WHEN** LLM mengembalikan `confidence = 0.60` untuk sebuah expense
- **THEN** backend merespons dengan HTTP 200 dan body
  `{"reply_type": "clarify", "persisted": false, "reply_text": "<question>"}`
- **AND** tidak ada baris yang ditulis ke tabel `transactions`

#### Scenario: High-confidence transactional parse is persisted

- **WHEN** LLM mengembalikan `confidence >= 0.75` untuk sebuah expense
  atau income
- **THEN** sebuah baris ditulis ke `transactions`
- **AND** body response memiliki `reply_type = "confirm"` dan
  `persisted = true`

#### Scenario: Unknown intent is always clarified

- **WHEN** `intent = unknown` terlepas dari nilai confidence
- **THEN** response berupa `reply_type = clarify`
- **AND** tidak ada apa pun yang dipersist

#### Scenario: set_goal atau set_budget tanpa amount valid

- **WHEN** intent `set_goal` atau `set_budget` di-parse dengan
  `amount = 0`
- **THEN** response berupa `reply_type = clarify` dengan
  `reply_text` yang meminta user menyebutkan nominal
- **AND** tidak ada goal/budget yang dibuat

### Requirement: Response Envelope

Body response dari `POST /api/message` (untuk request yang lolos
validasi) WAJIB mengikuti envelope tagged union berikut:

```
{
  "reply_type": "confirm" | "clarify" | "info" | "error",
  "persisted": <bool>,
  "reply_text": <string non-kosong>,
  // field tambahan tergantung reply_type:
  "transaction": { ... },   // hanya saat reply_type = "confirm"
  "context":     { ... },   // hanya saat reply_type = "confirm"
  "data":        { ... }    // hanya saat reply_type = "info"
}
```

`reply_text` WAJIB hadir dan non-kosong di setiap reply_type.
`persisted` WAJIB `true` HANYA saat `reply_type = confirm`; `false` di
semua reply_type lain.

#### Scenario: Confirm carries transaction and context

- **WHEN** sebuah expense atau income dipersist
- **THEN** body response berisi `reply_type = "confirm"`,
  `persisted = true`, `transaction`, `context`, dan `reply_text`
- **AND** field `data` TIDAK hadir

#### Scenario: Info carries data without transaction or context

- **WHEN** intent `balance`, `delete_last`, atau `check_goal` berhasil
- **THEN** body response berisi `reply_type = "info"`,
  `persisted = false`, `data`, dan `reply_text`
- **AND** field `transaction` dan `context` TIDAK hadir

#### Scenario: Clarify and error carry only reply_text

- **WHEN** `reply_type` adalah `clarify` atau `error`
- **THEN** body response hanya berisi `reply_type`, `persisted = false`,
  dan `reply_text`
- **AND** field `transaction`, `context`, dan `data` TIDAK hadir

### Requirement: Confirm Response Shape (expense/income persisted)

Saat sebuah transaksi dipersist, body response WAJIB menyertakan data
yang dibutuhkan untuk merender reply WhatsApp yang kontekstual tanpa
round-trip backend tambahan: saldo terkini, sisa budget untuk kategori
transaksi (jika ada budget), dan progress untuk setiap savings goal
aktif.

#### Scenario: Expense with active budget and goal

- **GIVEN** user memiliki budget aktif untuk `Makan & Minum` sebesar
  Rp 500.000 dan savings goal aktif `Laptop` di progress 32%
- **WHEN** sebuah expense Rp 45.000 di `Makan & Minum` dipersist
- **THEN** body response berisi
  ```json
  {
    "reply_type": "confirm",
    "persisted": true,
    "transaction": {
      "id": "<uuid>",
      "amount": 45000,
      "category": "Makan & Minum",
      "type": "expense"
    },
    "context": {
      "balance": 2455000,
      "category_budget": {
        "category": "Makan & Minum",
        "spent": 305000,
        "limit": 500000,
        "remaining": 195000
      },
      "savings_goals": [
        {
          "name": "Laptop",
          "saved": 3200000,
          "target": 10000000,
          "progress_pct": 32
        }
      ]
    },
    "reply_text": "<ringkasan natural-language hasil composer>"
  }
  ```

#### Scenario: Expense without a budget for that category

- **GIVEN** user belum men-set budget untuk `Hiburan`
- **WHEN** sebuah expense di `Hiburan` dipersist
- **THEN** `context.category_budget` bernilai `null`
- **AND** field konteks lainnya tetap terisi

#### Scenario: User has no active savings goals

- **WHEN** user tidak memiliki goal aktif
- **THEN** `context.savings_goals` adalah array kosong `[]`
- **AND** field tetap hadir (tidak dihilangkan)

#### Scenario: Composer fallback when LLM compose fails

- **GIVEN** transaksi sudah dipersist sukses
- **WHEN** panggilan composer ke LLM gagal (error/timeout)
- **THEN** backend tetap merespons `reply_type = "confirm"` dengan
  `transaction` dan `context` yang lengkap
- **AND** `reply_text` diisi template fallback minimal yang berisi
  ringkasan deterministik (mis. "✅ Tercatat: <category> Rp <amount>.
  Saldo: Rp <balance>.")
- **AND** error composer dilog

### Requirement: Info Response Shape (query intents)

Untuk intent informasional yang sukses dieksekusi (`balance`,
`delete_last`, `check_goal`), body response WAJIB `reply_type = info`
dengan `data` yang shape-nya ditentukan per intent.

#### Scenario: Balance query

- **WHEN** intent `balance` dieksekusi sukses
- **THEN** body response berisi
  ```json
  {
    "reply_type": "info",
    "persisted": false,
    "data": {"balance": <int64>},
    "reply_text": "<template: 'Saldo kamu Rp <balance>'>"
  }
  ```

#### Scenario: Delete last successful

- **WHEN** intent `delete_last` mengeksekusi penghapusan sukses
- **THEN** body response berisi
  ```json
  {
    "reply_type": "info",
    "persisted": false,
    "data": {
      "deleted": {
        "id": "<uuid>",
        "amount": <int64>,
        "category": "<string>",
        "type": "expense" | "income"
      }
    },
    "reply_text": "<template: '🗑️ Transaksi dihapus: ...'>"
  }
  ```

#### Scenario: Delete last when no history

- **WHEN** intent `delete_last` dijalankan dan tidak ada transaksi
- **THEN** body response berisi `reply_type = "info"` dan
  `data.deleted = null`
- **AND** `reply_text` template "Tidak ada transaksi yang bisa
  dihapus."

#### Scenario: Check goal returns active goals

- **WHEN** intent `check_goal` dieksekusi
- **THEN** body response berisi
  ```json
  {
    "reply_type": "info",
    "persisted": false,
    "data": {
      "savings_goals": [
        {"name": "<string>", "saved": <int64>, "target": <int64>, "progress_pct": <int>}
      ]
    },
    "reply_text": "<template summary semua goals>"
  }
  ```
- **AND** `data.savings_goals` adalah `[]` (bukan null) saat tidak ada
  goal aktif

#### Scenario: Report intent (placeholder MVP)

- **WHEN** intent `report` di-parse
- **THEN** body response berisi
  `{"reply_type": "clarify", "persisted": false, "reply_text": "Fitur laporan belum tersedia, mohon tunggu update ✨"}`
- **AND** tidak ada apa pun yang dipersist

### Requirement: Error Response Shape

Setelah parse LLM sukses, jika tahap berikutnya gagal (save DB error,
panggilan composer error untuk path non-confirm, port lain timeout
atau error), endpoint WAJIB merespons dengan HTTP 200 dan body
`{"reply_type": "error", "persisted": false, "reply_text": "<fallback>"}`.
Backend WAJIB log detail error untuk observability.

HTTP 4xx hanya digunakan untuk:
- 400 — request body malformed (sebelum parse)
- 404 — user tidak terdaftar (sebelum parse)

HTTP 5xx WAJIB TIDAK pernah dikembalikan dari endpoint ini di kondisi
parse sudah sukses.

#### Scenario: DB error after parse

- **GIVEN** parse LLM sukses dengan confidence tinggi untuk sebuah
  expense
- **WHEN** repo `transaction_repo.Save` mengembalikan error koneksi DB
  (bukan unique-violation)
- **THEN** body response `{"reply_type": "error", "persisted": false, "reply_text": "Maaf, ada gangguan sebentar. Coba lagi sebentar lagi 🙏"}`
- **AND** detail error dilog dengan stack trace
- **AND** HTTP status code = 200

#### Scenario: Composer error in confirm path does NOT trigger error

- **GIVEN** transaksi expense sudah dipersist sukses
- **WHEN** panggilan composer LLM error
- **THEN** response tetap `reply_type = "confirm"` (lihat scenario
  "Composer fallback when LLM compose fails")
- **AND** TIDAK menjadi `reply_type = "error"`

### Requirement: Raw Message Audit

Setiap transaksi yang dipersist WAJIB menyimpan teks asli WhatsApp apa
adanya di kolom `raw_message`. Teks WAJIB TIDAK dinormalisasi, di-trim
melebihi whitespace luar, atau dipotong.

#### Scenario: Raw text is preserved

- **WHEN** user mengirim `"  makan siang sama TEMEN 45rb!! 🍜  "`
- **AND** parse berhasil dengan confidence tinggi
- **THEN** `transactions.raw_message` tepat berisi
  `"makan siang sama TEMEN 45rb!! 🍜"` (hanya whitespace luar yang
  dihilangkan)

### Requirement: Idempotency by Received Timestamp

Backend WAJIB melakukan deduplikasi pesan inbound berdasarkan
`(user_id, received_at)` sehingga delivery webhook yang di-retry WAJIB
TIDAK membuat baris transaksi kedua. Migration menambahkan
`UNIQUE INDEX transactions_dedupe ON transactions (user_id, received_at)`.

#### Scenario: Duplicate webhook delivery returns same transaction

- **WHEN** n8n mem-post `(phone, received_at, message)` yang sama dua
  kali (untuk intent expense/income)
- **THEN** hanya ada satu baris di `transactions`
- **AND** kedua response memiliki `reply_type = "confirm"` dengan
  `transaction.id` yang sama

#### Scenario: Duplicate webhook re-computes context

- **GIVEN** retry webhook tiba setelah user sempat membuat transaksi
  lain di antara dua delivery
- **WHEN** dedup match `(user_id, received_at)` di repo
- **THEN** `context.balance` dan `context.category_budget.remaining`
  di response kedua merefleksikan state DB terbaru (bukan state saat
  delivery pertama)
- **AND** `reply_text` dipanggil ulang ke composer dengan context baru
