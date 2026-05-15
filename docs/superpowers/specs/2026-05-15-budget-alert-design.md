# Budget Alert — Inline Post-Transaction Design

## Goal

Menampilkan peringatan otomatis di pesan konfirmasi transaksi ketika pengeluaran bulan ini sudah menggerus ≥ 80% dari "ruang belanja" — yaitu sisa pemasukan setelah dikurangi alokasi nabung bulanan dari semua savings goal aktif yang punya deadline.

## Formula

```
monthly_savings_target = Σ (goal.TargetAmount - goal.SavedAmount) ÷ bulan_sampai_deadline
                         untuk semua active goal dengan Deadline != nil

ruang_belanja          = pemasukan_bulan_ini - monthly_savings_target

spending_alert         = pengeluaran_bulan_ini ≥ 80% × ruang_belanja
```

### Contoh

User punya 2 savings goal aktif dengan deadline:
- Jalan-jalan: target Rp 6.000.000, sudah Rp 0, deadline 12 bulan → Rp 500.000/bulan
- Tabungan darurat: target Rp 4.800.000, sudah Rp 0, deadline 24 bulan → Rp 200.000/bulan

Pemasukan bulan ini: Rp 10.000.000  
Total alokasi nabung: Rp 700.000/bulan  
Ruang belanja: Rp 9.300.000  
Threshold 80%: Rp 7.440.000  

Jika pengeluaran bulan ini ≥ Rp 7.440.000 → `SpendingAlert = true` → peringatan masuk ke reply WA.

## Komponen yang Berubah

### 1. `internal/domain/transaction.go`

Tambah method `GetMonthIncome` ke interface `TransactionRepository`:

```go
type TransactionRepository interface {
    Save(ctx context.Context, tx *Transaction) error
    DeleteLast(ctx context.Context, userID uuid.UUID) (*Transaction, error)
    GetBalance(ctx context.Context, userID uuid.UUID) (int64, error)
    GetTodaySpendByCategory(ctx context.Context, userID uuid.UUID) (map[string]int64, error)
    GetMonthSpendByCategory(ctx context.Context, userID uuid.UUID, year, month int) (map[string]int64, error)
    GetMonthIncome(ctx context.Context, userID uuid.UUID, year, month int) (int64, error)
}
```

### 2. `internal/platform/postgres/transaction_repo.go`

Implementasi `GetMonthIncome` — pola sama dengan `GetBalance`:

```go
func (r *transactionRepo) GetMonthIncome(ctx context.Context, userID uuid.UUID, year, month int) (int64, error) {
    var total int64
    err := r.db.WithContext(ctx).
        Model(&domain.Transaction{}).
        Select("COALESCE(SUM(amount), 0)").
        Where("user_id = ? AND type = 'income' AND EXTRACT(YEAR FROM date) = ? AND EXTRACT(MONTH FROM date) = ?",
            userID, year, month).
        Scan(&total).Error
    return total, err
}
```

### 3. `internal/domain/reminder.go`

Tambah field `SpendingAlert bool` ke `ReminderContext`:

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
    SpendingAlert   bool // true = pengeluaran ≥ 80% dari ruang belanja
}
```

Zero-value `false` — backward compatible dengan semua kode yang sudah ada.

### 4. `internal/usecase/reminder_usecase.go`

Di `BuildContext()`, setelah `goals` di-fetch, tambah kalkulasi:

**Step A: hitung monthly savings target dari goals**

```go
now := time.Now()
var monthlySavingsTarget int64
for _, g := range goals {
    if g.Deadline == nil || !g.IsActive {
        continue
    }
    remaining := g.TargetAmount - g.SavedAmount
    if remaining <= 0 {
        continue
    }
    months := (g.Deadline.Year()-now.Year())*12 + int(g.Deadline.Month()) - int(now.Month())
    if months <= 0 {
        continue
    }
    monthlySavingsTarget += remaining / int64(months)
}
```

**Step B: fetch income bulan ini**

```go
monthIncome, err := u.txRepo.GetMonthIncome(ctx, user.ID, now.Year(), int(now.Month()))
if err != nil {
    return nil, err
}
```

**Step C: hitung total pengeluaran bulan ini dan set alert**

```go
var monthExpense int64
for _, v := range monthByCategory {
    monthExpense += v
}

spendingAlert := false
if monthIncome > 0 && monthlySavingsTarget > 0 {
    spendingRoom := monthIncome - monthlySavingsTarget
    if spendingRoom > 0 {
        spendingAlert = monthExpense*100/spendingRoom >= 80
    }
}
```

Lalu tambahkan ke return:

```go
return &domain.ReminderContext{
    // ... existing fields ...
    SpendingAlert: spendingAlert,
}, nil
```

### 5. `internal/platform/llm/prompt.go`

Di `buildPostTransactionPrompt()`, tambah peringatan setelah bagian savings goal:

```go
if rc.SpendingAlert {
    sb.WriteString("- PERINGATAN: Pengeluaran bulan ini sudah ≥ 80% dari ruang belanja (pemasukan dikurangi alokasi nabung). Sertakan peringatan hemat dalam pesanmu.\n")
}
```

Import yang dibutuhkan sudah ada (`fmt`, `strings`, `time`, `budgeting/internal/domain`, `budgeting/pkg/currency`).

## Edge Cases

| Kondisi | Behaviour |
|---------|-----------|
| Semua goal tidak punya deadline | `monthlySavingsTarget = 0` → skip alert (tidak ada konteks nabung) |
| Goal sudah melewati deadline | `months <= 0` → goal dilewati |
| Goal sudah tercapai (`SavedAmount >= TargetAmount`) | `remaining <= 0` → goal dilewati |
| `pemasukan_bulan_ini = 0` | `monthIncome = 0` → skip alert (guard `monthIncome > 0`) |
| `ruang_belanja ≤ 0` (alokasi nabung > pemasukan) | Skip alert (guard `spendingRoom > 0`) |
| Intent `income` bukan expense | `monthExpense` tidak berubah banyak → kemungkinan alert tetap terhitung tapi jarang trigger |

## Testing

### Integration test `GetMonthIncome` — `transaction_repo_test.go`

- Seed 2 income transactions bulan ini → `GetMonthIncome` return jumlah keduanya
- Seed income bulan lalu → tidak masuk hasil bulan ini
- Tidak ada income bulan ini → return 0 (bukan error)

### Unit test `BuildContext` alert logic — `reminder_usecase_test.go`

Semua test menggunakan fake repo (tidak butuh DB):

- **Alert muncul**: income Rp 10jt, savings target Rp 700rb, expense Rp 7.5jt → `SpendingAlert = true` (80.6%)
- **Alert tidak muncul**: expense Rp 7.4jt → `SpendingAlert = false` (79.5%)
- **Goal tanpa deadline**: `monthlySavingsTarget = 0` → skip alert → `SpendingAlert = false`
- **Income nol**: `monthIncome = 0` → skip alert → `SpendingAlert = false`
- **Ruang belanja negatif**: income Rp 500rb, savings target Rp 700rb → skip alert → `SpendingAlert = false`

### Unit test prompt builder — `prompt_test.go` (file baru)

- `buildPostTransactionPrompt` dengan `SpendingAlert = true` → output mengandung "PERINGATAN"
- `buildPostTransactionPrompt` dengan `SpendingAlert = false` → output tidak mengandung "PERINGATAN"

### Regression

`go test ./...` harus tetap hijau — `SpendingAlert` zero-value `false` di semua fake repo yang sudah ada.
