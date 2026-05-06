# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

Aplikasi budgeting personal berbasis **WhatsApp** sebagai antarmuka utama untuk mencatat transaksi dan menerima reminder. Detail laporan & dashboard dilihat via **aplikasi Android (Flutter)**. Tidak ada web frontend.

User boleh menulis pesan bebas — LLM yang bertugas memahami intent dan mengkategorikan secara otomatis.

## Tech Stack

| Layer | Teknologi | Peran |
|---|---|---|
| Messaging interface | WhatsApp Business API | Input transaksi & notifikasi reminder |
| Workflow automation | n8n (self-hosted) | Terima webhook WA, forward raw text, scheduling reminder |
| Backend API | Golang | Business logic, REST API, autentikasi |
| LLM Understanding | Claude API (Anthropic) | Parse & kategorisasi pesan bebas dari user |
| Database | PostgreSQL | Penyimpanan transaksi & user |
| Mobile | Flutter (Android) | Dashboard, laporan, grafik |

## Architecture

### Aliran Data Utama

```
[WhatsApp User]
      │ pesan bebas: "makan siang sama temen 45rb"
      ▼
[n8n — WhatsApp Webhook]
      │ forward raw text + phone number
      │ HTTP POST /api/message
      ▼
[Golang — Message Handler]
      │
      ▼
[LLM Understanding Service]  ← pkg/understanding/
      │ kirim raw text ke Claude API
      │ structured output: type, amount, category, note, confidence
      │
      ├─ confidence tinggi → langsung simpan ke DB
      └─ confidence rendah → balas WA minta konfirmasi
      ▼
[Service + Repository]
      │ simpan transaksi
      ▼
[PostgreSQL]
      │
      ◄── HTTP GET ── [Flutter App (Android)]
```

### LLM Understanding Layer

**Lokasi**: `backend/pkg/understanding/`

LLM menerima pesan mentah dan mengembalikan structured JSON:

```json
{
  "intent": "expense",
  "amount": 45000,
  "category": "Makan & Minum",
  "note": "makan siang sama temen",
  "confidence": 0.95
}
```

**Intent yang dikenali LLM:**

| Intent | Deskripsi |
|---|---|
| `expense` | pengeluaran |
| `income` | pemasukan |
| `balance` | cek saldo |
| `report` | minta laporan |
| `delete_last` | hapus transaksi terakhir |
| `unknown` | tidak dikenali, minta klarifikasi |

**Confidence threshold**: jika `< 0.75`, Golang balas ke user via n8n untuk konfirmasi sebelum menyimpan.

**Kategori yang diketahui LLM (di-inject ke system prompt):**

```
Pengeluaran: Makan & Minum, Transport, Belanja, Tagihan, Hiburan, Kesehatan, Pendidikan, Lainnya
Pemasukan:   Gaji, Freelance, Investasi, Hadiah, Lainnya
```

LLM bebas memetakan kata apapun ke kategori ini. Contoh:
- "bakso", "kopi", "warteg" → `Makan & Minum`
- "grab", "bensin", "parkir" → `Transport`
- "netflix", "bioskop" → `Hiburan`

### Reminder Flow

```
[n8n — Cron Trigger]
      │ harian/mingguan
      ▼
[Golang — GET /api/reminder/:phone]
      │ ambil: saldo, sisa budget per kategori, progress savings goal
      │
      ▼
[LLM Understanding Service]
      │ susun pesan reminder yang natural & personal
      │ sertakan motivasi berhemat jika ada savings goal aktif
      ▼
[n8n] → kirim pesan reminder ke WhatsApp user
```

**Dua jenis reminder:**

| Jenis | Trigger | Isi |
|---|---|---|
| Post-transaction | Setiap kali transaksi dicatat | Konfirmasi + sisa budget kategori + saldo + progress goal |
| Daily reminder | n8n cron (misal jam 20.00) | Ringkasan hari ini + sisa budget bulan ini + saldo + motivasi goal |

**Contoh pesan post-transaction:**
```
✅ Tercatat: Makan siang Rp 45.000

💳 Saldo: Rp 2.455.000
📊 Budget Makan & Minum: Rp 305.000 / 500.000 (sisa Rp 195.000)
🎯 Target Laptop Rp 10jt — terkumpul Rp 3.200.000 (32%) — yuk hemat!
```

**Contoh pesan daily reminder:**
```
📅 Ringkasan hari ini, Selasa 29 April

Pengeluaran: Rp 125.000
  • Makan & Minum Rp 85.000
  • Transport Rp 40.000

💰 Saldo: Rp 2.455.000
📊 Sisa budget bulan ini: Rp 1.230.000

🎯 Target Laptop Rp 10jt
   Terkumpul: Rp 3.200.000 (32%)
   Kalau hemat Rp 500rb/bulan, bisa tercapai dalam ~14 bulan 💪
```

### Struktur Direktori (Target)

Backend mengikuti **Clean Architecture** dengan 4 layer — dependency hanya boleh mengalir ke dalam (handler → usecase → domain ← platform):

```
budgeting/
├── backend/
│   ├── cmd/
│   │   └── server/
│   │       └── main.go                     # DI wiring + start HTTP server
│   │
│   ├── internal/
│   │   │
│   │   ├── domain/                         # Layer 1 — Core (zero external deps)
│   │   │   ├── transaction.go              # entity Transaction + interface TransactionRepository
│   │   │   ├── user.go                     # entity User + interface UserRepository
│   │   │   ├── savings_goal.go             # entity SavingsGoal + interface SavingsGoalRepository
│   │   │   ├── budget_target.go            # entity BudgetTarget + interface BudgetTargetRepository
│   │   │   └── reminder.go                 # struct ReminderContext + interface MessageComposer
│   │   │
│   │   ├── usecase/                        # Layer 2 — Business logic (depends on domain interfaces)
│   │   │   ├── transaction_usecase.go      # RecordTransaction, DeleteLast, GetBalance
│   │   │   ├── reminder_usecase.go         # BuildDailyReminder, BuildPostTransactionReply
│   │   │   └── goal_usecase.go             # SetGoal, SetBudget, CheckProgress
│   │   │
│   │   ├── handler/                        # Layer 3 — HTTP delivery (depends on usecase)
│   │   │   ├── router.go                   # route registration
│   │   │   ├── message_handler.go          # POST /api/message
│   │   │   ├── reminder_handler.go         # GET /api/reminder/:phone
│   │   │   └── goal_handler.go             # POST /api/goals, POST /api/budgets
│   │   │
│   │   └── platform/                       # Layer 4 — Infrastructure (implements domain interfaces)
│   │       ├── postgres/
│   │       │   ├── db.go                   # connection pool setup
│   │       │   ├── transaction_repo.go     # implements TransactionRepository
│   │       │   ├── user_repo.go            # implements UserRepository
│   │       │   ├── savings_goal_repo.go    # implements SavingsGoalRepository
│   │       │   └── budget_target_repo.go   # implements BudgetTargetRepository
│   │       └── llm/
│   │           ├── client.go               # Claude API call wrapper
│   │           ├── prompt.go               # system prompt builder + kategori + intents
│   │           ├── reminder_composer.go    # implements MessageComposer
│   │           └── parser_test.go          # test variasi input WA user
│   │
│   ├── pkg/                                # Utilities reusable lintas project
│   │   ├── currency/                       # FormatIDR(amount int64) string
│   │   └── httputil/                       # JSONResponse, ErrorResponse helpers
│   │
│   └── migrations/
│       ├── 001_users.sql
│       ├── 002_transactions.sql
│       ├── 003_savings_goals.sql
│       └── 004_budget_targets.sql
│
├── mobile/
│   └── lib/
│       ├── features/
│       │   ├── dashboard/                  # ringkasan saldo + progress goals
│       │   ├── history/                    # daftar transaksi + filter
│       │   └── report/                     # grafik & breakdown kategori
│       ├── core/
│       │   ├── api/                        # HTTP client ke Golang backend
│       │   └── models/                     # data model dari API response
│       └── main.dart
│
└── n8n/
    └── workflows/
        ├── wa-inbound.json                 # terima WA → POST /api/message → balas WA
        └── reminder.json                   # cron harian → GET /api/reminder/:phone → kirim WA
```

**Aturan dependency antar layer:**

| Layer | Boleh depend on | Dilarang depend on |
|---|---|---|
| `domain` | — (tidak boleh import apapun dari internal) | semua layer lain |
| `usecase` | `domain` | `handler`, `platform` |
| `handler` | `usecase`, `domain` | `platform` |
| `platform` | `domain` | `usecase`, `handler` |

### Data Model Utama

```go
type Transaction struct {
    ID         uuid.UUID  `db:"id"`
    UserID     uuid.UUID  `db:"user_id"`
    Type       string     `db:"type"`       // "income" | "expense"
    Amount     int64      `db:"amount"`     // dalam Rupiah
    Category   string     `db:"category"`
    Note       string     `db:"note"`
    Date       time.Time  `db:"date"`
    CreatedAt  time.Time  `db:"created_at"`
    RawMessage string     `db:"raw_message"` // pesan asli user, untuk audit
}

type ParseResult struct {
    Intent     string  `json:"intent"`
    Amount     int64   `json:"amount"`
    Category   string  `json:"category"`
    Note       string  `json:"note"`
    Confidence float64 `json:"confidence"`
}

// SavingsGoal — target tabungan user (misal: "beli laptop Rp 10jt")
type SavingsGoal struct {
    ID          uuid.UUID  `db:"id"`
    UserID      uuid.UUID  `db:"user_id"`
    Name        string     `db:"name"`         // "Laptop", "Liburan Bali"
    TargetAmount int64     `db:"target_amount"` // dalam Rupiah
    SavedAmount int64      `db:"saved_amount"`  // akumulasi dari income bertag "tabungan"
    Deadline    *time.Time `db:"deadline"`      // opsional
    IsActive    bool       `db:"is_active"`
    CreatedAt   time.Time  `db:"created_at"`
}

// BudgetTarget — batas pengeluaran per kategori per bulan
type BudgetTarget struct {
    ID        uuid.UUID `db:"id"`
    UserID    uuid.UUID `db:"user_id"`
    Category  string    `db:"category"`
    Amount    int64     `db:"amount"` // batas Rupiah per bulan
    Month     int       `db:"month"`  // 1-12
    Year      int       `db:"year"`
}

// ReminderContext — data yang dikirim ke LLM untuk susun pesan reminder
type ReminderContext struct {
    UserName      string
    Balance       int64
    TodaySpend    int64
    SpendByCategory map[string]int64
    BudgetRemaining map[string]int64  // sisa budget per kategori
    SavingsGoals  []SavingsGoal
}
```

**Intent tambahan untuk LLM:**

| Intent | Contoh pesan user | Aksi |
|---|---|---|
| `set_goal` | "mau nabung buat laptop 10 juta" | Buat SavingsGoal baru |
| `set_budget` | "budget makan bulan ini 500rb" | Buat/update BudgetTarget |
| `check_goal` | "progress nabung laptop gimana?" | Tampilkan progress SavingsGoal |

### Prinsip Arsitektur

- **n8n hanya forward, tidak parse**: n8n tidak boleh melakukan logika apapun terhadap isi pesan — cukup teruskan raw text + phone number ke Golang.
- **LLM Understanding ada di backend (`pkg/understanding`)**: lebih mudah di-test, retry, dan diganti model tanpa ubah n8n.
- **Simpan `raw_message`**: pesan asli user selalu disimpan untuk keperluan audit dan debug.
- **Confidence gate**: LLM dengan confidence rendah wajib konfirmasi dulu ke user, tidak langsung simpan.
- **Flutter read-only**: semua mutasi data hanya via WhatsApp, Flutter hanya display.
- **Clean Architecture 4 layer**: `domain` → `usecase` → `handler` + `platform`; dependency hanya boleh mengalir ke dalam menuju `domain`. Handler tidak boleh akses DB atau LLM langsung.
- **LLM susun pesan reminder**: Golang hanya siapkan data (ReminderContext), LLM yang merangkai teks reminder agar natural dan motivatif — bukan template hardcode.
- **Post-transaction reply selalu sertakan konteks**: setiap transaksi berhasil dicatat, reply ke WA wajib sertakan saldo terkini + sisa budget kategori tersebut + progress savings goal aktif (jika ada).
- **Reminder harian dikelola n8n cron**: n8n hit `GET /api/reminder/:phone` lalu forward balasan ke WA. Golang + LLM yang menyusun isi pesannya.

## Golang Commands

```bash
cd backend
go run ./cmd/server
go test ./...
go test ./pkg/understanding/   # test LLM parser dengan berbagai variasi input
go build -o bin/server ./cmd/server
```

## Flutter Commands

```bash
cd mobile
flutter pub get
flutter run
flutter test
flutter build apk --release
```

## Roadmap

- [ ] MVP: LLM parse pesan WA → catat transaksi → reply saldo + sisa budget
- [ ] Confidence gate: konfirmasi via WA jika LLM tidak yakin
- [ ] Savings goal: user set target via WA ("mau nabung buat laptop 10 juta")
- [ ] Budget target per kategori: user set via WA ("budget makan bulan ini 500rb")
- [ ] Post-transaction reply: saldo + sisa budget kategori + progress savings goal
- [ ] Daily reminder via WA (n8n cron jam 20.00): ringkasan + motivasi goal
- [ ] Flutter dashboard: saldo + history + progress savings goal
- [ ] Laporan mingguan otomatis via WA
- [ ] Alert WA otomatis jika budget kategori hampir habis (> 80%)
- [ ] Multi-user (keluarga)
