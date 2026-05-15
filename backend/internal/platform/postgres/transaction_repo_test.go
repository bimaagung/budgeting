package postgres_test

import (
	"context"
	"os"
	"testing"
	"time"

	"budgeting/internal/domain"
	"budgeting/internal/platform/postgres"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// newTestDB returns a connected GORM DB or skips the test.
// Requires DATABASE_URL_TEST pointing at a Postgres where migrations
// (including 005_transactions_dedupe.sql) have been applied.
func newTestDB(t *testing.T) (*gorm.DB, func()) {
	t.Helper()
	url := os.Getenv("DATABASE_URL_TEST")
	if url == "" {
		t.Skip("DATABASE_URL_TEST not set; integration test skipped")
	}
	prev := os.Getenv("DATABASE_URL")
	_ = os.Setenv("DATABASE_URL", url)
	db, err := postgres.NewDB()
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	cleanup := func() { _ = os.Setenv("DATABASE_URL", prev) }
	return db, cleanup
}

func mustSeedUser(t *testing.T, db *gorm.DB) uuid.UUID {
	t.Helper()
	u := domain.User{ID: uuid.New(), Phone: "+62" + uuid.New().String()[:10], Name: "test"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return u.ID
}

func TestTransactionRepo_DoubleInsertSameReceivedAt(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewTransactionRepository(db)
	uid := mustSeedUser(t, db)
	rcv := time.Now().UTC().Truncate(time.Second)

	tx1 := domain.Transaction{
		ID: uuid.New(), UserID: uid, Type: "expense", Amount: 45000,
		Category: "Makan & Minum", Date: time.Now(), ReceivedAt: rcv, RawMessage: "makan",
	}
	if err := repo.Save(context.Background(), &tx1); err != nil {
		t.Fatalf("first save: %v", err)
	}
	firstID := tx1.ID

	tx2 := domain.Transaction{
		ID: uuid.New(), UserID: uid, Type: "expense", Amount: 99999,
		Category: "Hiburan", Date: time.Now(), ReceivedAt: rcv, RawMessage: "duplicate",
	}
	if err := repo.Save(context.Background(), &tx2); err != nil {
		t.Fatalf("second save (should swallow unique-violation): %v", err)
	}
	if tx2.ID != firstID {
		t.Errorf("after dedup, tx2.ID = %s, want firstID %s (existing row should be returned)", tx2.ID, firstID)
	}
	if tx2.Amount != 45000 {
		t.Errorf("after dedup, tx2.Amount = %d, want 45000 (first insert wins)", tx2.Amount)
	}

	var count int64
	if err := db.Model(&domain.Transaction{}).
		Where("user_id = ? AND received_at = ?", uid, rcv).
		Count(&count).Error; err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("rows in DB: got %d, want 1", count)
	}
}

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
