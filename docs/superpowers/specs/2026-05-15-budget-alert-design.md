# Budget Alert — Inline Post-Transaction Design

## Goal

Menampilkan peringatan otomatis di pesan konfirmasi transaksi ketika pengeluaran suatu kategori sudah mencapai ≥ 80% dari budget bulan ini, tanpa perlu kirim pesan WA terpisah.

## Arsitektur

Logika deteksi "hampir habis" dihitung di Go (`BuildContext`), bukan di LLM. Go menentukan kategori mana yang melewati threshold, lalu meneruskan daftarnya ke prompt builder. LLM hanya bertugas merangkai pesan yang natural — termasuk peringatan jika diminta.

Perubahan hanya di 3 file kecil. Tidak ada endpoint baru, tidak ada perubahan n8n, tidak ada perubahan skema DB.

## Threshold

Pengeluaran bulan ini ≥ 80% dari `BudgetTarget.Amount` untuk kategori yang sama di bulan dan tahun yang sama.

Formula: `monthSpend * 100 / budgetAmount >= 80`

## Komponen yang Berubah

### 1. `internal/domain/reminder.go`

Tambah field `NearBudgetLimit []string` ke struct `ReminderContext`:

```go
type ReminderContext struct {
    UserName        string
    Balance         int64
    LastCategory    string
    LastAmount      int64
    TodaySpend      int64
    SpendByCategory map[string]int64
    BudgetRemaining map[string]int64
    SavingsGoals    []SavingsGoal
    NearBudgetLimit []string // kategori dengan pengeluaran bulan ini ≥ 80% budget
}
```

Field ini zero-value (`nil`) jika tidak ada kategori yang hampir habis — backward compatible dengan semua kode yang sudah ada.

### 2. `internal/usecase/reminder_usecase.go`

Di `BuildContext()`, setelah loop `budgetRemaining`, tambah kalkulasi `NearBudgetLimit`:

```go
var nearBudgetLimit []string
for _, b := range budgets {
    spent := monthByCategory[b.Category]
    if b.Amount > 0 && spent*100/b.Amount >= 80 {
        nearBudgetLimit = append(nearBudgetLimit, b.Category)
    }
}
```

Lalu tambahkan field ke return value:

```go
return &domain.ReminderContext{
    // ... existing fields ...
    NearBudgetLimit: nearBudgetLimit,
}, nil
```

Kategori hanya masuk ke `NearBudgetLimit` jika:
- Ada `BudgetTarget` untuk kategori itu di bulan/tahun ini (`b.Amount > 0`)
- Pengeluaran bulan ini ≥ 80% dari budget tersebut

Kategori tanpa budget yang ditetapkan tidak menghasilkan alert.

### 3. `internal/platform/llm/prompt.go`

Di fungsi `buildPostTransactionPrompt()`, tambah section peringatan setelah bagian savings goal:

```go
if len(rc.NearBudgetLimit) > 0 {
    sb.WriteString(fmt.Sprintf("- PERINGATAN budget hampir habis: %s\n",
        strings.Join(rc.NearBudgetLimit, ", ")))
    sb.WriteString("Sertakan peringatan ini dalam pesan konfirmasi.\n")
}
```

Import `strings` sudah ada di file ini.

## Data Flow

```
User: "makan siang 45rb"
  │
  ▼
handleTransaction() — save expense Makan & Minum Rp 45.000
  │
  ▼
BuildContext()
  ├─ monthByCategory["Makan & Minum"] = 430.000  (bulan ini total)
  ├─ budgets["Makan & Minum"].Amount  = 500.000  (budget ditetapkan)
  ├─ 430.000 * 100 / 500.000 = 86  ≥ 80 → masuk NearBudgetLimit
  └─ ReminderContext.NearBudgetLimit = ["Makan & Minum"]
  │
  ▼
buildPostTransactionPrompt()
  └─ "- PERINGATAN budget hampir habis: Makan & Minum"
  │
  ▼
LLM generate reply:
  "✅ Tercatat: Makan & Minum Rp 45.000
   ⚠️ Budget Makan & Minum hampir habis (86% terpakai)!
   💰 Saldo: Rp 1.955.000"
```

## Edge Cases

| Kondisi | Behaviour |
|---------|-----------|
| Kategori tidak punya budget | Tidak masuk `NearBudgetLimit` (tidak ada alert) |
| Budget = 0 | Dilewati (guard `b.Amount > 0`) |
| Tepat 80% | Alert muncul (threshold `>= 80`) |
| Tepat 100% (habis) | Alert tetap muncul |
| Multiple kategori hampir habis | Semua masuk list, LLM sebut semuanya |
| Intent `income` (bukan expense) | `BuildContext` tetap dipanggil, tapi pemasukan tidak menambah `monthSpend` — alert tidak muncul untuk income |

## Testing

### Unit test `BuildContext` — `reminder_usecase_test.go`

- Seed budget Rp 500.000 untuk kategori "Makan & Minum", month spend Rp 400.000 (80%) → `NearBudgetLimit` berisi "Makan & Minum"
- Spend Rp 399.000 (79.8%) → `NearBudgetLimit` kosong
- Kategori tanpa budget → tidak masuk list meski spend besar
- Budget = 0 → tidak panic, tidak masuk list

### Unit test prompt builder — `prompt_test.go` (file baru)

- `buildPostTransactionPrompt` dengan `NearBudgetLimit = ["Makan & Minum"]` → output mengandung "PERINGATAN" dan "Makan & Minum"
- `buildPostTransactionPrompt` dengan `NearBudgetLimit = nil` → output tidak mengandung "PERINGATAN"

### Regression

- `go test ./...` harus tetap hijau — `NearBudgetLimit` nil-safe di semua kode yang sudah ada
