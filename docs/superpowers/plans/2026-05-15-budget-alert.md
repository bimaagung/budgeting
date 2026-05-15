# Budget Alert Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Tambahkan peringatan otomatis di pesan konfirmasi transaksi ketika pengeluaran bulan ini sudah ≥ 80% dari "ruang belanja" (pemasukan bulan ini dikurangi total alokasi tabungan dari savings goals aktif yang punya deadline).

**Architecture:** `TransactionRepository` ditambah `GetMonthIncome` untuk ambil data income bulan ini. `ReminderContext` ditambah `SpendingAlert bool`. `BuildContext` di `ReminderUsecase` menghitung alert setelah fetch income + goals. `buildPostTransactionPrompt` menyisipkan instruksi peringatan ke LLM jika `SpendingAlert = true`.

**Tech Stack:** Go 1.21+, GORM, PostgreSQL, testify (tidak ada — gunakan stdlib `testing`), Fiber

---

## File Map

| File | Aksi |
|------|------|
| `backend/internal/domain/transaction.go` | Modify — tambah `GetMonthIncome` ke interface |
| `backend/internal/platform/postgres/transaction_repo.go` | Modify — implementasi `GetMonthIncome` |
| `backend/internal/platform/postgres/transaction_repo_test.go` | Modify — tambah 3 integration test untuk `GetMonthIncome` |
| `backend/internal/usecase/message_usecase_test.go` | Modify — tambah `GetMonthIncome` ke `fakeTxRepo` dan `dedupAwareTxRepo` |
| `backend/internal/domain/reminder.go` | Modify — tambah `SpendingAlert bool` ke `ReminderContext` |
| `backend/internal/usecase/reminder_usecase.go` | Modify — hitung dan set `SpendingAlert` di `BuildContext` |
| `backend/internal/usecase/reminder_usecase_test.go` | Create — unit tests untuk logika alert di `BuildContext` |
| `backend/internal/platform/llm/prompt.go` | Modify — sisipkan blok PERINGATAN ke `buildPostTransactionPrompt` |
| `backend/internal/platform/llm/prompt_test.go` | Create — unit tests `buildPostTransactionPrompt` dengan/tanpa alert |

---

### Task 1: GetMonthIncome — interface + implementasi + integration test

**Files:**
- Modify: `backend/internal/domain/transaction.go`
- Modify: `backend/internal/platform/postgres/transaction_repo.go`
- Modify: `backend/internal/platform/postgres/transaction_repo_test.go`
- Modify: `backend/internal/usecase/message_usecase_test.go`

- [ ] **Step 1: Tambah `GetMonthIncome` ke interface `TransactionRepository`**

Edit `backend/internal/domain/transaction.go`. Ganti blok interface:

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

- [ ] **Step 2: Tambah `GetMonthIncome` ke `fakeTxRepo` dan `dedupAwareTxRepo` di test**

Edit `backend/internal/usecase/message_usecase_test.go`.

Tambah field `monthIncome int64` ke struct `fakeTxRepo`:

```go
type fakeTxRepo struct {
	saved        []domain.Transaction
	saveErr      error
	balance      int64
	deleteResult *domain.Transaction
	monthSpend   map[string]int64
	todaySpend   map[string]int64
	monthIncome  int64
}
```

Tambah method ke `fakeTxRepo`:

```go
func (f *fakeTxRepo) GetMonthIncome(_ context.Context, _ uuid.UUID, _, _ int) (int64, error) {
	return f.monthIncome, nil
}
```

Tambah method ke `dedupAwareTxRepo`:

```go
func (r *dedupAwareTxRepo) GetMonthIncome(_ context.Context, _ uuid.UUID, _, _ int) (int64, error) {
	return 0, nil
}
```

- [ ] **Step 3: Verifikasi kompilasi bersih setelah interface dan fake diupdate**

```bash
cd backend && go build ./...
```

Expected: tidak ada error kompilasi.

- [ ] **Step 4: Tulis failing integration test untuk `GetMonthIncome`**

Tambah di akhir `backend/internal/platform/postgres/transaction_repo_test.go`:

```go
func TestGetMonthIncome_ReturnsSumForCurrentMonth(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewTransactionRepository(db)
	uid := mustSeedUser(t, db)
	now := time.Now()

	seeds := []domain.Transaction{
		{ID: uuid.New(), UserID: uid, Type: "income", Amount: 3_000_000,
			Category: "Gaji", Date: now, ReceivedAt: now.Add(-2 * time.Second), RawMessage: "gaji"},
		{ID: uuid.New(), UserID: uid, Type: "income", Amount: 500_000,
			Category: "Freelance", Date: now, ReceivedAt: now.Add(-1 * time.Second), RawMessage: "freelance"},
		// expense — harus diabaikan
		{ID: uuid.New(), UserID: uid, Type: "expense", Amount: 100_000,
			Category: "Makan & Minum", Date: now, ReceivedAt: now, RawMessage: "makan"},
	}
	for i := range seeds {
		if err := db.Create(&seeds[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	total, err := repo.GetMonthIncome(context.Background(), uid, now.Year(), int(now.Month()))
	if err != nil {
		t.Fatalf("GetMonthIncome: %v", err)
	}
	if total != 3_500_000 {
		t.Errorf("total = %d, want 3500000", total)
	}
}

func TestGetMonthIncome_ExcludesPreviousMonth(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewTransactionRepository(db)
	uid := mustSeedUser(t, db)
	now := time.Now()
	prevMonth := now.AddDate(0, -1, 0)

	seeds := []domain.Transaction{
		// income bulan lalu — harus diabaikan
		{ID: uuid.New(), UserID: uid, Type: "income", Amount: 5_000_000,
			Category: "Gaji", Date: prevMonth, ReceivedAt: prevMonth, RawMessage: "gaji lalu"},
		// income bulan ini
		{ID: uuid.New(), UserID: uid, Type: "income", Amount: 1_000_000,
			Category: "Gaji", Date: now, ReceivedAt: now.Add(-1 * time.Second), RawMessage: "gaji ini"},
	}
	for i := range seeds {
		if err := db.Create(&seeds[i]).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	total, err := repo.GetMonthIncome(context.Background(), uid, now.Year(), int(now.Month()))
	if err != nil {
		t.Fatalf("GetMonthIncome: %v", err)
	}
	if total != 1_000_000 {
		t.Errorf("total = %d, want 1000000", total)
	}
}

func TestGetMonthIncome_ReturnsZeroWhenNoIncome(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewTransactionRepository(db)
	uid := mustSeedUser(t, db)
	now := time.Now()

	total, err := repo.GetMonthIncome(context.Background(), uid, now.Year(), int(now.Month()))
	if err != nil {
		t.Fatalf("GetMonthIncome: %v", err)
	}
	if total != 0 {
		t.Errorf("total = %d, want 0", total)
	}
}
```

- [ ] **Step 5: Jalankan test — pastikan FAIL karena method belum ada**

```bash
cd backend && DATABASE_URL_TEST="postgres://postgres:123456@localhost:5432/budgeting_test?sslmode=disable" go test ./internal/platform/postgres/ -run TestGetMonthIncome -v
```

Expected: compile error — `GetMonthIncome` tidak ada di `transactionRepo`.

- [ ] **Step 6: Implementasi `GetMonthIncome` di `transaction_repo.go`**

Tambah method ini di akhir `backend/internal/platform/postgres/transaction_repo.go`:

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

- [ ] **Step 7: Jalankan integration test — pastikan PASS**

```bash
cd backend && DATABASE_URL_TEST="postgres://postgres:123456@localhost:5432/budgeting_test?sslmode=disable" go test ./internal/platform/postgres/ -run TestGetMonthIncome -v
```

Expected: 3 test PASS.

- [ ] **Step 8: Pastikan seluruh test suite masih hijau**

```bash
cd backend && go test ./...
```

Expected: PASS semua (integration test skip jika `DATABASE_URL_TEST` tidak di-set).

- [ ] **Step 9: Commit**

```bash
git add backend/internal/domain/transaction.go \
        backend/internal/platform/postgres/transaction_repo.go \
        backend/internal/platform/postgres/transaction_repo_test.go \
        backend/internal/usecase/message_usecase_test.go
git commit -m "feat(repo): add GetMonthIncome to TransactionRepository"
```

---

### Task 2: SpendingAlert — ReminderContext + BuildContext logic + unit tests

**Files:**
- Modify: `backend/internal/domain/reminder.go`
- Modify: `backend/internal/usecase/reminder_usecase.go`
- Create: `backend/internal/usecase/reminder_usecase_test.go`

- [ ] **Step 1: Tambah `SpendingAlert bool` ke `ReminderContext`**

Edit `backend/internal/domain/reminder.go`. Ganti struct:

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
	SpendingAlert   bool
}
```

- [ ] **Step 2: Tulis failing unit test untuk logika alert di `BuildContext`**

Buat file baru `backend/internal/usecase/reminder_usecase_test.go`:

```go
package usecase

import (
	"context"
	"testing"
	"time"

	"budgeting/internal/domain"

	"github.com/google/uuid"
)

// fakeGoalRepoR is a separate fake to avoid conflict with fakeGoalRepo in message_usecase_test.go.
// Both live in package usecase (same test binary), so we need distinct names.
type fakeGoalRepoR struct {
	goals []domain.SavingsGoal
}

func (f *fakeGoalRepoR) Create(_ context.Context, g domain.SavingsGoal) error {
	f.goals = append(f.goals, g)
	return nil
}
func (f *fakeGoalRepoR) GetActiveByUser(_ context.Context, _ uuid.UUID) ([]domain.SavingsGoal, error) {
	return f.goals, nil
}

func deadlineMonthsFromNow(n int) *time.Time {
	t := time.Now().AddDate(0, n, 0)
	return &t
}

func newReminderUC(txRepo domain.TransactionRepository, goals []domain.SavingsGoal) *ReminderUsecase {
	gr := &fakeGoalRepoR{goals: goals}
	br := &fakeBudgetRepo{}
	ur := &fakeUserRepo{user: &domain.User{ID: uuid.New(), Name: "test", Phone: "+62x"}}
	return NewReminderUsecase(txRepo, gr, br, ur, &fakeComposer{})
}

func TestBuildContext_SpendingAlert_TriggersAt80Percent(t *testing.T) {
	// income 10jt, savings target 700rb (500rb + 200rb), ruang belanja 9.3jt
	// expense 7.5jt = 80.6% → alert harus true
	deadline12 := deadlineMonthsFromNow(12)
	deadline24 := deadlineMonthsFromNow(24)
	goals := []domain.SavingsGoal{
		{ID: uuid.New(), Name: "Jalan-jalan", TargetAmount: 6_000_000, SavedAmount: 0, Deadline: deadline12, IsActive: true},
		{ID: uuid.New(), Name: "Darurat", TargetAmount: 4_800_000, SavedAmount: 0, Deadline: deadline24, IsActive: true},
	}
	tx := &fakeTxRepo{
		monthIncome: 10_000_000,
		monthSpend:  map[string]int64{"Makan & Minum": 4_000_000, "Transport": 3_500_000},
	}
	uc := newReminderUC(tx, goals)
	user := &domain.User{ID: uuid.New(), Name: "test"}
	rc, err := uc.BuildContext(context.Background(), user, "", 0)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if !rc.SpendingAlert {
		t.Errorf("SpendingAlert = false, want true (expense 7.5jt ≥ 80%% of ruang belanja 9.3jt)")
	}
}

func TestBuildContext_SpendingAlert_DoesNotTriggerBelow80(t *testing.T) {
	// income 10jt, savings target 700rb, ruang belanja 9.3jt
	// expense 7.4jt = 79.5% → alert harus false
	deadline12 := deadlineMonthsFromNow(12)
	deadline24 := deadlineMonthsFromNow(24)
	goals := []domain.SavingsGoal{
		{ID: uuid.New(), Name: "Jalan-jalan", TargetAmount: 6_000_000, SavedAmount: 0, Deadline: deadline12, IsActive: true},
		{ID: uuid.New(), Name: "Darurat", TargetAmount: 4_800_000, SavedAmount: 0, Deadline: deadline24, IsActive: true},
	}
	tx := &fakeTxRepo{
		monthIncome: 10_000_000,
		monthSpend:  map[string]int64{"Makan & Minum": 4_000_000, "Transport": 3_400_000},
	}
	uc := newReminderUC(tx, goals)
	user := &domain.User{ID: uuid.New(), Name: "test"}
	rc, err := uc.BuildContext(context.Background(), user, "", 0)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if rc.SpendingAlert {
		t.Errorf("SpendingAlert = true, want false (expense 7.4jt < 80%% of ruang belanja 9.3jt)")
	}
}

func TestBuildContext_SpendingAlert_SkipsWhenNoGoalWithDeadline(t *testing.T) {
	// goal tanpa deadline → monthlySavingsTarget = 0 → skip alert
	goals := []domain.SavingsGoal{
		{ID: uuid.New(), Name: "Santai", TargetAmount: 5_000_000, SavedAmount: 0, Deadline: nil, IsActive: true},
	}
	tx := &fakeTxRepo{
		monthIncome: 10_000_000,
		monthSpend:  map[string]int64{"Makan & Minum": 9_500_000},
	}
	uc := newReminderUC(tx, goals)
	user := &domain.User{ID: uuid.New(), Name: "test"}
	rc, err := uc.BuildContext(context.Background(), user, "", 0)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if rc.SpendingAlert {
		t.Errorf("SpendingAlert = true, want false (no goal with deadline → no target → skip alert)")
	}
}

func TestBuildContext_SpendingAlert_SkipsWhenIncomeZero(t *testing.T) {
	deadline12 := deadlineMonthsFromNow(12)
	goals := []domain.SavingsGoal{
		{ID: uuid.New(), Name: "Laptop", TargetAmount: 10_000_000, SavedAmount: 0, Deadline: deadline12, IsActive: true},
	}
	tx := &fakeTxRepo{
		monthIncome: 0,
		monthSpend:  map[string]int64{"Makan & Minum": 5_000_000},
	}
	uc := newReminderUC(tx, goals)
	user := &domain.User{ID: uuid.New(), Name: "test"}
	rc, err := uc.BuildContext(context.Background(), user, "", 0)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if rc.SpendingAlert {
		t.Errorf("SpendingAlert = true, want false (monthIncome = 0 → skip alert)")
	}
}

func TestBuildContext_SpendingAlert_SkipsWhenSpendingRoomNegative(t *testing.T) {
	// income 500rb, savings target 700rb → spendingRoom = -200rb → skip alert
	deadline12 := deadlineMonthsFromNow(12)
	goals := []domain.SavingsGoal{
		{ID: uuid.New(), Name: "Laptop", TargetAmount: 8_400_000, SavedAmount: 0, Deadline: deadline12, IsActive: true},
	}
	tx := &fakeTxRepo{
		monthIncome: 500_000,
		monthSpend:  map[string]int64{"Makan & Minum": 300_000},
	}
	uc := newReminderUC(tx, goals)
	user := &domain.User{ID: uuid.New(), Name: "test"}
	rc, err := uc.BuildContext(context.Background(), user, "", 0)
	if err != nil {
		t.Fatalf("BuildContext: %v", err)
	}
	if rc.SpendingAlert {
		t.Errorf("SpendingAlert = true, want false (spendingRoom ≤ 0 → skip alert)")
	}
}
```

- [ ] **Step 3: Jalankan test — pastikan FAIL karena logika belum ada**

```bash
cd backend && go test ./internal/usecase/ -run TestBuildContext_SpendingAlert -v
```

Expected: 5 test FAIL — `SpendingAlert` selalu false (field ada tapi belum di-set).

- [ ] **Step 4: Implementasi logika alert di `BuildContext`**

Edit `backend/internal/usecase/reminder_usecase.go`. Di dalam `BuildContext`, setelah baris `goals, err := u.goalRepo.GetActiveByUser(...)` dan sebelum blok `budgetRemaining`, tambahkan kalkulasi berikut:

```go
	// Hitung monthly savings target dari active goals dengan deadline
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

	monthIncome, err := u.txRepo.GetMonthIncome(ctx, user.ID, now.Year(), int(now.Month()))
	if err != nil {
		return nil, err
	}

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

Lalu tambah `SpendingAlert: spendingAlert` ke return value:

```go
	return &domain.ReminderContext{
		UserName:        user.Name,
		Balance:         balance,
		LastCategory:    lastCategory,
		LastAmount:      lastAmount,
		TodaySpend:      todayTotal,
		SpendByCategory: todayByCategory,
		BudgetRemaining: budgetRemaining,
		SavingsGoals:    goals,
		SpendingAlert:   spendingAlert,
	}, nil
```

- [ ] **Step 5: Jalankan unit test — pastikan PASS**

```bash
cd backend && go test ./internal/usecase/ -run TestBuildContext_SpendingAlert -v
```

Expected: 5 test PASS.

- [ ] **Step 6: Pastikan seluruh test suite masih hijau**

```bash
cd backend && go test ./...
```

Expected: semua PASS.

- [ ] **Step 7: Commit**

```bash
git add backend/internal/domain/reminder.go \
        backend/internal/usecase/reminder_usecase.go \
        backend/internal/usecase/reminder_usecase_test.go
git commit -m "feat(usecase): compute SpendingAlert in BuildContext based on savings goal allocation"
```

---

### Task 3: Prompt update — PERINGATAN di buildPostTransactionPrompt + prompt test

**Files:**
- Modify: `backend/internal/platform/llm/prompt.go`
- Create: `backend/internal/platform/llm/prompt_test.go`

- [ ] **Step 1: Tulis failing test untuk `buildPostTransactionPrompt`**

Buat file baru `backend/internal/platform/llm/prompt_test.go`:

```go
package llm

import (
	"strings"
	"testing"

	"budgeting/internal/domain"
)

func TestBuildPostTransactionPrompt_ContainsPERINGATANWhenAlertTrue(t *testing.T) {
	rc := domain.ReminderContext{
		LastCategory:  "Makan & Minum",
		LastAmount:    50_000,
		Balance:       2_000_000,
		SpendingAlert: true,
	}
	out := buildPostTransactionPrompt(rc)
	if !strings.Contains(out, "PERINGATAN") {
		t.Errorf("output should contain PERINGATAN when SpendingAlert=true, got:\n%s", out)
	}
}

func TestBuildPostTransactionPrompt_NoPERINGATANWhenAlertFalse(t *testing.T) {
	rc := domain.ReminderContext{
		LastCategory:  "Makan & Minum",
		LastAmount:    50_000,
		Balance:       2_000_000,
		SpendingAlert: false,
	}
	out := buildPostTransactionPrompt(rc)
	if strings.Contains(out, "PERINGATAN") {
		t.Errorf("output should NOT contain PERINGATAN when SpendingAlert=false, got:\n%s", out)
	}
}
```

- [ ] **Step 2: Jalankan test — pastikan FAIL**

```bash
cd backend && go test ./internal/platform/llm/ -run TestBuildPostTransactionPrompt -v
```

Expected: `TestBuildPostTransactionPrompt_ContainsPERINGATANWhenAlertTrue` FAIL — output belum mengandung "PERINGATAN".

- [ ] **Step 3: Tambah blok peringatan ke `buildPostTransactionPrompt`**

Edit `backend/internal/platform/llm/prompt.go`. Di dalam `buildPostTransactionPrompt`, setelah blok `len(rc.SavingsGoals) > 0` dan sebelum `sb.WriteString("\nSusun pesan...")`, tambahkan:

```go
	if rc.SpendingAlert {
		sb.WriteString("- PERINGATAN: Pengeluaran bulan ini sudah ≥ 80% dari ruang belanja (pemasukan dikurangi alokasi nabung). Sertakan peringatan hemat dalam pesanmu.\n")
	}
```

Fungsi lengkap setelah perubahan:

```go
func buildPostTransactionPrompt(rc domain.ReminderContext) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("Transaksi baru dicatat:\n- Kategori: %s\n- Jumlah: Rp %s\n- Saldo sekarang: Rp %s\n",
		rc.LastCategory, currency.FormatIDR(rc.LastAmount), currency.FormatIDR(rc.Balance)))

	if sisa, ok := rc.BudgetRemaining[rc.LastCategory]; ok {
		sb.WriteString(fmt.Sprintf("- Sisa budget %s bulan ini: Rp %s\n", rc.LastCategory, currency.FormatIDR(sisa)))
	}

	if len(rc.SavingsGoals) > 0 {
		g := rc.SavingsGoals[0]
		pct := int64(0)
		if g.TargetAmount > 0 {
			pct = g.SavedAmount * 100 / g.TargetAmount
		}
		sb.WriteString(fmt.Sprintf("- Target tabungan '%s': Rp %s / Rp %s (%d%%)\n",
			g.Name, currency.FormatIDR(g.SavedAmount), currency.FormatIDR(g.TargetAmount), pct))
	}

	if rc.SpendingAlert {
		sb.WriteString("- PERINGATAN: Pengeluaran bulan ini sudah ≥ 80% dari ruang belanja (pemasukan dikurangi alokasi nabung). Sertakan peringatan hemat dalam pesanmu.\n")
	}

	sb.WriteString("\nSusun pesan konfirmasi transaksi + konteks keuangan di atas. Singkat dan motivatif.")
	return sb.String()
}
```

- [ ] **Step 4: Jalankan test — pastikan PASS**

```bash
cd backend && go test ./internal/platform/llm/ -run TestBuildPostTransactionPrompt -v
```

Expected: 2 test PASS.

- [ ] **Step 5: Pastikan seluruh test suite masih hijau**

```bash
cd backend && go test ./...
```

Expected: semua PASS. Zero-value `SpendingAlert = false` backward-compatible dengan semua fake repo yang sudah ada.

- [ ] **Step 6: Commit**

```bash
git add backend/internal/platform/llm/prompt.go \
        backend/internal/platform/llm/prompt_test.go
git commit -m "feat(llm): inject spending alert warning into post-transaction prompt"
```
