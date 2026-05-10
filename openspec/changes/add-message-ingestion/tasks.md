## 1. Pin kontrak HTTP

- [ ] 1.1 Tambah field `received_at` (string RFC3339) ke request struct di `internal/handler/message_handler.go`; reject HTTP 400 saat hilang atau format invalid.
- [ ] 1.2 Refactor `getOrCreateUser` di `internal/usecase/message_usecase.go` menjadi `userRepo.FindByPhone` strict; user tidak ditemukan → return error sentinel yang di-mapping handler ke HTTP 404 `{"error": "user not registered"}`.
- [ ] 1.3 Implement phone normalizer (`+62`/`62`/`0` → E.164 `+62`) di `internal/handler/`; reject HTTP 400 untuk format yang tidak dikenali.
- [ ] 1.4 Tambahkan fixture JSON schema di `internal/handler/testdata/message_request.schema.json` dan handler-level test untuk: payload well-formed, missing field per kombinasi, format `received_at` invalid, format phone invalid, user tidak terdaftar.

## 2. Validasi output parse LLM

- [ ] 2.1 Buat `internal/platform/llm/parser.go` dengan helper `Validate(raw []byte) (domain.ParseResult, error)` yang menegakkan field set, enum `intent`, `amount >= 0`, dan `0 <= confidence <= 1`.
- [ ] 2.2 Pasang `Validate` di `composer.ParseMessage` setelah panggilan API; saat gagal, log raw output dan kembalikan `ParseResult{Intent: "unknown", Confidence: 0}`.
- [ ] 2.3 Perluas `internal/platform/llm/parser_test.go` dengan kasus table-driven untuk setiap bentuk malformed (field hilang, tipe salah, confidence di luar range, intent tidak dikenal, amount negatif, JSON invalid).

## 3. Tegakkan confidence gate di usecase

- [ ] 3.1 Di `internal/usecase/message_usecase.go`, arahkan `confidence < 0.75` (atau `intent = unknown`) ke jalur klarifikasi yang mengembalikan `reply_type = "clarify"` dan tidak persist.
- [ ] 3.2 Untuk `intent = set_goal` atau `set_budget` dengan `amount = 0`, arahkan ke jalur klarifikasi (`reply_type = "clarify"`).
- [ ] 3.3 Hapus pengecekan confidence duplikat dari layer handler (jika ada).
- [ ] 3.4 Tambahkan test usecase yang mencakup: persist confidence tinggi, klarifikasi confidence rendah, klarifikasi unknown intent, klarifikasi set_goal tanpa amount.

## 4. Persist `raw_message` apa adanya

- [ ] 4.1 Trim hanya whitespace luar (`strings.TrimSpace`) sebelum meneruskan pesan ke parser; simpan nilai yang sudah di-trim ke `transactions.raw_message`.
- [ ] 4.2 Tambahkan test usecase yang memastikan emoji dan whitespace internal tetap bertahan setelah round-trip.

## 5. Build response envelope (tagged union)

- [ ] 5.1 Definisikan struct response tagged union di `internal/handler/dto.go` (atau `internal/usecase/result.go` kalau lebih cocok); marshaling JSON harus menjamin field opsional benar-benar omit (gunakan pointer atau `omitempty` dengan struct kosong).
- [ ] 5.2 Confirm path (expense/income persist sukses): build `transaction` + `context` (saldo, category_budget, savings_goals) + panggil composer LLM untuk `reply_text`.
- [ ] 5.3 Confirm path — composer error → fallback template deterministik untuk `reply_text` (mis. "✅ Tercatat: <category> Rp <amount>. Saldo: Rp <balance>."); reply_type tetap `confirm` karena data sudah dipersist.
- [ ] 5.4 Info path (`balance`, `delete_last`, `check_goal`): build `data` per intent + `reply_text` via template deterministik.
- [ ] 5.5 Clarify path: `reply_text` via template deterministik per sub-skenario (low-confidence, unknown intent, missing amount, report belum tersedia).
- [ ] 5.6 Error path: setiap error post-parse non-confirm → `reply_type = "error"` + `reply_text` fallback tetap; backend log detail error.
- [ ] 5.7 Test usecase: assert bentuk response per `reply_type` terhadap fixture JSON schema.

## 6. Idempotency

- [ ] 6.1 Tambahkan migration `005_transactions_dedupe.sql` yang membuat `CREATE UNIQUE INDEX transactions_dedupe ON transactions (user_id, received_at)`.
- [ ] 6.2 Di `internal/platform/postgres/transaction_repo.go`, perlakukan unique-violation saat insert sebagai "sudah dipersist" dan kembalikan baris yang sudah ada (tanpa error).
- [ ] 6.3 Di usecase, untuk path dedup-hit, tetap re-build `context` dari state terbaru dan re-call composer untuk `reply_text` baru (tidak cache response pertama).
- [ ] 6.4 Tambahkan test integrasi level repo yang assert double-insert mengembalikan id yang sama dan hanya satu baris yang ada.
- [ ] 6.5 Tambahkan test usecase yang assert: retry dengan transaksi lain di antara dua delivery → `transaction.id` sama tapi `context.balance` merefleksikan state baru.

## 7. Report intent stub

- [ ] 7.1 Di usecase, intent `report` → `reply_type = "clarify"` dengan `reply_text = "Fitur laporan belum tersedia, mohon tunggu update ✨"`.
- [ ] 7.2 Test memastikan `report` tidak persist apa pun.

## 8. Kontrak n8n

- [ ] 8.1 Update `n8n/workflows/wa-inbound.json` agar HTTP node meneruskan tepat `{phone, message, received_at}` dan menggunakan timestamp event WhatsApp Business API (bukan `Date.now()`).
- [ ] 8.2 Dokumentasikan kontrak via comment node di dalam JSON workflow untuk editor di masa depan.

## 9. Sinkronkan dokumentasi project

- [ ] 9.1 Update `CLAUDE.md` agar referensi `pkg/understanding/` berubah menjadi `internal/platform/llm/parser.go` (sesuai Decision 4 di design.md).
- [ ] 9.2 Update tabel "Pemetaan Spec ke Kode" di `openspec/project.md` jika ada path yang berubah.
- [ ] 9.3 Pastikan response envelope tagged union didokumentasikan ringkas di `CLAUDE.md` agar workflow change berikutnya tahu kontraknya.

## 10. Verifikasi

- [ ] 10.1 `cd backend && go test ./...` lulus.
- [ ] 10.2 `openspec validate add-message-ingestion --strict` lulus.
- [ ] 10.3 Smoke test manual: kirim pesan WA via test workflow n8n untuk minimal 4 skenario — expense well-formed, balance query, low-confidence input, payload missing field — dan amati baris di `transactions` plus envelope response yang tepat di response log.
