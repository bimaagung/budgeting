package usecase

import (
	"context"
	"testing"
	"time"

	"budgeting/internal/domain"

	"github.com/google/uuid"
)

func deadlineMonthsFromNow(n int) *time.Time {
	t := time.Now().AddDate(0, n, 0)
	return &t
}

func newReminderUC(txRepo domain.TransactionRepository, goals []domain.SavingsGoal) *ReminderUsecase {
	gr := &fakeGoalRepo{active: goals}
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
