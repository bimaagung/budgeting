## 1. Pin kontrak HTTP

- [x] 1.1 Tambah field `received_at` (string RFC3339) ke request struct di `internal/handler/message_handler.go`; reject HTTP 400 saat hilang atau format invalid.
- [x] 1.2 Refactor `getOrCreateUser` di `internal/usecase/message_usecase.go` menjadi `userRepo.FindByPhone` strict; user tidak ditemukan → return error sentinel yang di-mapping handler ke HTTP 404 `{"error": "user not registered"}`.
- [x] 1.3 Implement phone normalizer (`+62`/`62`/`0` → E.164 `+62`) di `internal/handler/`; reject HTTP 400 untuk format yang tidak dikenali.
- [x] 1.4 Tambahkan fixture JSON schema di `internal/handler/testdata/message_request.schema.json` dan handler-level test untuk: payload well-formed, missing field per kombinasi, format `received_at` invalid, format phone invalid, user tidak terdaftar.

## 2. Validasi output parse LLM

- [x] 2.1 Buat `internal/platform/llm/parser.go` dengan helper `Validate(raw []byte) (domain.ParseResult, error)` yang menegakkan field set, enum `intent`, `amount >= 0`, dan `0 <= confidence <= 1`.
- [x] 2.2 Pasang `Validate` di `composer.ParseMessage` setelah panggilan API; saat gagal, log raw output dan kembalikan `ParseResult{Intent: "unknown", Confidence: 0}`.
- [x] 2.3 Perluas `internal/platform/llm/parser_validate_test.go` dengan kasus table-driven untuk setiap bentuk malformed (field hilang, tipe salah, confidence di luar range, intent tidak dikenal, amount negatif, JSON invalid).

## 3. Tegakkan confidence gate di usecase

- [x] 3.1 Di `internal/usecase/message_usecase.go`, arahkan `confidence < 0.75` (atau `intent = unknown`) ke jalur klarifikasi yang mengembalikan `reply_type = "clarify"` dan tidak persist.
- [x] 3.2 Untuk `intent = set_goal` atau `set_budget` dengan `amount = 0`, arahkan ke jalur klarifikasi (`reply_type = "clarify"`).
- [x] 3.3 Hapus pengecekan confidence duplikat dari layer handler (jika ada).
- [x] 3.4 Tambahkan test usecase yang mencakup: persist confidence tinggi, klarifikasi confidence rendah, klarifikasi unknown intent, klarifikasi set_goal tanpa amount.

## 4. Persist `raw_message` apa adanya

- [x] 4.1 Trim hanya whitespace luar (`strings.TrimSpace`) sebelum meneruskan pesan ke parser; simpan nilai yang sudah di-trim ke `transactions.raw_message`.
- [x] 4.2 Tambahkan test usecase yang memastikan emoji dan whitespace internal tetap bertahan setelah round-trip.

## 5. Build response envelope (tagged union)

- [x] 5.1 Definisikan struct response tagged union di `internal/handler/dto.go` (atau `internal/usecase/result.go` kalau lebih cocok); marshaling JSON harus menjamin field opsional benar-benar omit (gunakan pointer atau `omitempty` dengan struct kosong).
- [x] 5.2 Confirm path (expense/income persist sukses): build `transaction` + `context` (saldo, category_budget, savings_goals) + panggil composer LLM untuk `reply_text`.
- [x] 5.3 Confirm path — composer error → fallback template deterministik untuk `reply_text` (mis. "✅ Tercatat: <category> Rp <amount>. Saldo: Rp <balance>."); reply_type tetap `confirm` karena data sudah dipersist.
- [x] 5.4 Info path (`balance`, `delete_last`, `check_goal`): build `data` per intent + `reply_text` via template deterministik.
- [x] 5.5 Clarify path: `reply_text` via template deterministik per sub-skenario (low-confidence, unknown intent, missing amount, report belum tersedia).
- [x] 5.6 Error path: setiap error post-parse non-confirm → `reply_type = "error"` + `reply_text` fallback tetap; backend log detail error.
- [x] 5.7 Test usecase: assert bentuk response per `reply_type` terhadap fixture JSON schema.

## 6. Idempotency

- [x] 6.1 Tambahkan migration `005_transactions_dedupe.sql` yang membuat `CREATE UNIQUE INDEX transactions_dedupe ON transactions (user_id, received_at)`.
- [x] 6.2 Di `internal/platform/postgres/transaction_repo.go`, perlakukan unique-violation saat insert sebagai "sudah dipersist" dan kembalikan baris yang sudah ada (tanpa error).
- [x] 6.3 Di usecase, untuk path dedup-hit, tetap re-build `context` dari state terbaru dan re-call composer untuk `reply_text` baru (tidak cache response pertama).
- [x] 6.4 Tambahkan test integrasi level repo yang assert double-insert mengembalikan id yang sama dan hanya satu baris yang ada.
- [x] 6.5 Tambahkan test usecase yang assert: retry dengan transaksi lain di antara dua delivery → `transaction.id` sama tapi `context.balance` merefleksikan state baru.

## 7. Report intent stub

- [x] 7.1 Di usecase, intent `report` → `reply_type = "clarify"` dengan `reply_text = "Fitur laporan belum tersedia, mohon tunggu update ✨"`.
- [x] 7.2 Test memastikan `report` tidak persist apa pun.

## 8. Kontrak n8n

- [x] 8.1 Update `n8n/workflows/wa-inbound.json` agar HTTP node meneruskan tepat `{phone, message, received_at}` dan menggunakan timestamp event WhatsApp Business API (bukan `Date.now()`).
- [x] 8.2 Dokumentasikan kontrak via comment node di dalam JSON workflow untuk editor di masa depan.

## 9. Sinkronkan dokumentasi project

- [x] 9.1 Update `CLAUDE.md` agar referensi `pkg/understanding/` berubah menjadi `internal/platform/llm/parser.go` (sesuai Decision 4 di design.md).
- [x] 9.2 Update tabel "Pemetaan Spec ke Kode" di `openspec/project.md` jika ada path yang berubah.
- [x] 9.3 Pastikan response envelope tagged union didokumentasikan ringkas di `CLAUDE.md` agar workflow change berikutnya tahu kontraknya.

## 10. Verifikasi

- [x] 10.1 `cd backend && go test ./...` lulus.
- [ ] 10.2 `openspec validate add-message-ingestion --strict` lulus.
- [ ] 10.3 Smoke test manual: kirim pesan WA via test workflow n8n untuk minimal 4 skenario — expense well-formed, balance query, low-confidence input, payload missing field — dan amati baris di `transactions` plus envelope response yang tepat di response log.
