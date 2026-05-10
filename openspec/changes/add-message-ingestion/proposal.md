## Why

Produk bergantung pada user yang mengetik pesan WhatsApp bebas (misal
"makan siang sama temen 45rb") dan pesan tersebut diubah menjadi
transaksi terstruktur atau jawaban query. Saat ini alur tersebut belum
terdokumentasi sebagai kontrak: n8n, backend Go, dan LLM masing-masing
membuat asumsi yang saling berbeda. Tanpa spec, kita tidak bisa menjamin
bahwa:

- n8n meneruskan bentuk payload yang stabil (saat ini bergantung pada
  apa yang terakhir kali diedit oleh penulis workflow).
- Skema output LLM tetap konsisten saat versi model berubah.
- Parse dengan confidence rendah selalu memicu reply konfirmasi alih-alih
  diam-diam menulis baris yang salah.
- n8n hanya perlu membaca satu shape response untuk semua jenis pesan
  (transaksi, query saldo, hapus, set goal/budget, dst).
- Kegagalan internal (DB, LLM down) tidak meninggalkan user dalam
  kebisuan karena backend mengembalikan 5xx.

Change ini menetapkan kapabilitas message-ingestion sebagai spec OpenSpec
pertama agar change berikutnya (komposisi reminder, lifecycle savings
goal, lifecycle budget target, dll.) berdiri di atas fondasi yang
terdokumentasi.

## What Changes

- Definisikan kontrak request **`POST /api/message`** (`phone`,
  `message`, `received_at`) sehingga n8n punya target tetap dan backend
  bisa menolak payload malformed.
- Definisikan **response envelope tagged union** (`reply_type` ∈
  `confirm`, `clarify`, `info`, `error`) yang seragam untuk semua 9
  intent yang masuk via endpoint ini.
- Spesifikasikan **skema output parse LLM** (`intent`, `amount`,
  `category`, `note`, `confidence`) dan nilai `intent` yang diizinkan.
- Spesifikasikan perilaku **confidence gate**: parse di bawah `0.75`
  atau `intent = unknown` tidak boleh dipersist; sebagai gantinya
  endpoint mengembalikan `reply_type = clarify` yang diteruskan oleh n8n
  ke user.
- Spesifikasikan **aturan audit raw_message**: setiap transaksi yang
  diterima menyimpan teks asli apa adanya (hanya whitespace luar yang
  di-trim).
- Spesifikasikan **shape response confirm** (untuk expense/income yang
  dipersist): saldo + sisa budget kategori + progress savings goal aktif,
  sehingga n8n bisa merender reply tanpa round-trip tambahan.
- Spesifikasikan **shape response info** (untuk balance, delete_last,
  check_goal): `data` per intent + `reply_text` template deterministik.
- Spesifikasikan **shape response error**: setelah parse sukses, error
  internal apa pun (DB save gagal, composer LLM down) → HTTP 200 dengan
  `reply_type = error` + `reply_text` fallback aman; backend log detail
  error untuk observability.
- Spesifikasikan **policy user registration**: endpoint TIDAK
  auto-create user; phone yang tidak match → HTTP 404. Onboarding via
  channel lain (out of scope change ini).
- Spesifikasikan **phone normalization**: handler menormalisasi inbound
  phone ke E.164 sebelum lookup user dan dedup, sehingga variasi format
  WA Business API tidak menyebabkan duplikat akun.
- Spesifikasikan **idempotency** dengan kunci `(user_id, received_at)`
  agar retry webhook aman; response duplicate tetap punya
  `transaction.id` sama tapi context dan reply_text di-recompute dari
  state terbaru.

Implementasi kode menyesuaikan kode yang sudah ada di backend; bagian
yang masih divergen (auto-create user, single response shape, no
phone normalization, no `received_at`, no idempotency) akan disinkronkan
melalui tasks change ini.

## Capabilities

### New Capabilities
- `message-ingestion`: Kontrak untuk menerima pesan WhatsApp mentah,
  parsing via LLM, menerapkan confidence gate, melakukan persistensi
  transaksi (untuk intent transaksional) atau menjawab query (untuk
  intent informasional), serta mengembalikan response envelope tagged
  union yang konsisten untuk semua 9 intent.

### Modified Capabilities
<!-- Belum ada — spec pertama di project. -->

## Impact

- **Code path yang dicakup:**
  - `internal/handler/message_handler.go` (request validation, phone
    normalization, response envelope marshaling)
  - `internal/usecase/message_usecase.go` (intent dispatch, confidence
    gate, build response per `reply_type`)
  - `internal/platform/llm/parser.go` (validator output LLM, baru)
  - `internal/platform/llm/composer.go` (panggilan LLM untuk
    `reply_type = confirm`)
  - `internal/platform/postgres/transaction_repo.go` (handle
    unique-violation sebagai existing-row)
  - `internal/platform/postgres/user_repo.go` (`FindByPhone` strict, no
    auto-create)
- **Kontrak eksternal yang di-pin:** request body workflow `wa-inbound`
  n8n, response shape yang dibaca n8n, skema structured output Claude
  API.
- **Database:** menambah migration `005_transactions_dedupe.sql`
  (`UNIQUE INDEX (user_id, received_at)`); selain itu read-only terhadap
  tabel `transactions`, `budget_targets`, `savings_goals`.
- **Observability:** field audit `raw_message` menjadi bagian dari spec.
  Semua error internal post-parse dilog sebelum di-coerce menjadi
  `reply_type = error`.
- **Di luar scope (change terpisah ke depan):** komposisi
  natural-language reply_text (own oleh kapabilitas
  `reminder-composition`), lifecycle savings-goal, lifecycle
  budget-target, intent `report` (akan return `clarify` "belum
  tersedia"), multi-user, rate-limiting.
