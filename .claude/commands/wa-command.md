# Skill: wa-command

Tambahkan kemampuan baru untuk memahami pesan WhatsApp user — mencakup LLM prompt, backend handler, dan test.

## Tugas

Tambahkan WA command/intent baru: **$ARGUMENTS**

## Implementasi

### 1. Update System Prompt (`backend/internal/platform/llm/prompt.go`)
- Tambahkan deskripsi intent baru ke system prompt LLM
- Berikan contoh variasi kalimat user yang harus dikenali
- Pastikan output JSON tetap mengikuti struct `ParseResult`

### 2. Update Intent Routing (`backend/internal/handler/message_handler.go`)
- Tambahkan case baru untuk intent yang baru di switch/if routing
- Panggil usecase yang sesuai

### 3. Usecase & Repository (jika butuh data baru)
- Tambahkan method di `internal/usecase/`
- Tambahkan interface di `internal/domain/` jika butuh data baru
- Tambahkan implementasi query di `internal/platform/postgres/`

### 4. Response WhatsApp
- Response harus singkat dan natural, cocok dibaca di WA
- Jika intent ini bisa ambigu, pastikan confidence gate aktif (threshold 0.75)
- Untuk intent `expense` dan `income`: **wajib sertakan ReminderContext** di reply:
  - Saldo terkini
  - Sisa budget kategori yang baru dicatat
  - Progress savings goal aktif (jika ada)

### 5. Intent Khusus: Goal & Budget

**`set_goal`** — user ingin menabung untuk sesuatu:
- Contoh: "mau nabung buat laptop 10 juta", "nabung liburan bali target 5jt"
- Output ParseResult: `intent: "set_goal"`, `note: "Laptop"`, `amount: 10000000`
- Handler: buat/update `SavingsGoal` di DB
- Reply: "🎯 Target *Laptop* Rp 10.000.000 sudah dicatat! Semangat nabung ya 💪"

**`set_budget`** — user ingin set batas pengeluaran per kategori:
- Contoh: "budget makan bulan ini 500rb", "limit transport 300 ribu"
- Output ParseResult: `intent: "set_budget"`, `category: "Makan & Minum"`, `amount: 500000`
- Handler: buat/update `BudgetTarget` di DB untuk bulan berjalan
- Reply: "✅ Budget *Makan & Minum* bulan ini diset Rp 500.000"

**`check_goal`** — user minta lihat progress tabungan:
- Contoh: "progress nabung gimana?", "udah nabung berapa buat laptop?"
- Handler: ambil semua SavingsGoal aktif, format via LLM
- Reply: tampilkan progress tiap goal dengan persentase dan estimasi waktu

### 6. Test (`backend/pkg/understanding/parser_test.go`)
Tulis test cases dengan berbagai variasi input natural:
```go
// Contoh test cases yang harus dicakup:
// - variasi ejaan: "mkn", "makan", "makan2"
// - urutan kata berbeda: "50rb makan" vs "makan 50rb"
// - nominal shorthand: "50rb", "50k", "50ribu", "50.000"
// - konteks tambahan: "makan sama temen", "makan di warung padang"
// - intent goal: "nabung laptop 10jt", "mau beli hp baru 3 juta"
// - intent budget: "limit makan 500rb", "budget transport bulan ini 200k"
```

## Konvensi
- LLM yang memutuskan kategori dan intent — jangan hardcode mapping kata ke kategori di kode
- Simpan selalu `raw_message` untuk audit
- Response ke user ramah jika tidak dikenali: "Maaf, saya kurang yakin maksudnya. Bisa tulis ulang? Contoh: makan 50rb"
- Setiap reply transaksi berhasil: selalu sertakan saldo + sisa budget + progress goal (panggil `reminder.BuildPostTransactionReply(ctx)` di service layer)
