package usecase

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"budgeting/internal/domain"

	"github.com/google/uuid"
)

// --- fakes ---

type fakeUserRepo struct{ user *domain.User }

func (f *fakeUserRepo) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	if f.user == nil {
		return nil, nil
	}
	return f.user, nil
}
func (f *fakeUserRepo) Upsert(ctx context.Context, u domain.User) (*domain.User, error) {
	f.user = &u
	return &u, nil
}

type fakeTxRepo struct {
	saved        []domain.Transaction
	saveErr      error
	balance      int64
	deleteResult *domain.Transaction
	monthSpend   map[string]int64
	todaySpend   map[string]int64
}

func (f *fakeTxRepo) Save(ctx context.Context, tx *domain.Transaction) error {
	if f.saveErr != nil {
		return f.saveErr
	}
	f.saved = append(f.saved, *tx)
	return nil
}
func (f *fakeTxRepo) DeleteLast(ctx context.Context, userID uuid.UUID) (*domain.Transaction, error) {
	return f.deleteResult, nil
}
func (f *fakeTxRepo) GetBalance(ctx context.Context, userID uuid.UUID) (int64, error) {
	return f.balance, nil
}
func (f *fakeTxRepo) GetTodaySpendByCategory(ctx context.Context, userID uuid.UUID) (map[string]int64, error) {
	return f.todaySpend, nil
}
func (f *fakeTxRepo) GetMonthSpendByCategory(ctx context.Context, userID uuid.UUID, year, month int) (map[string]int64, error) {
	return f.monthSpend, nil
}

type fakeGoalRepo struct {
	created []domain.SavingsGoal
	active  []domain.SavingsGoal
}

func (f *fakeGoalRepo) Create(ctx context.Context, g domain.SavingsGoal) error {
	f.created = append(f.created, g)
	return nil
}
func (f *fakeGoalRepo) GetActiveByUser(ctx context.Context, uid uuid.UUID) ([]domain.SavingsGoal, error) {
	return f.active, nil
}

type fakeBudgetRepo struct {
	upserted []domain.BudgetTarget
	byMonth  []domain.BudgetTarget
}

func (f *fakeBudgetRepo) Upsert(ctx context.Context, bt domain.BudgetTarget) error {
	f.upserted = append(f.upserted, bt)
	return nil
}
func (f *fakeBudgetRepo) GetByUserAndMonth(ctx context.Context, uid uuid.UUID, year, month int) ([]domain.BudgetTarget, error) {
	return f.byMonth, nil
}

type fakeComposer struct {
	parseResult *domain.ParseResult
	parseErr    error
	composeErr  error
	composeText string
}

func (f *fakeComposer) ParseMessage(ctx context.Context, raw string) (*domain.ParseResult, error) {
	return f.parseResult, f.parseErr
}
func (f *fakeComposer) FormatPostTransaction(ctx context.Context, rc domain.ReminderContext) (string, error) {
	if f.composeErr != nil {
		return "", f.composeErr
	}
	return f.composeText, nil
}
func (f *fakeComposer) FormatDailyReminder(ctx context.Context, rc domain.ReminderContext) (string, error) {
	return f.composeText, f.composeErr
}

// --- helpers ---

func newUsecase(t *testing.T, ur domain.UserRepository, tx domain.TransactionRepository,
	gr domain.SavingsGoalRepository, br domain.BudgetTargetRepository,
	c domain.MessageComposer) *MessageUsecase {
	t.Helper()
	rUC := NewReminderUsecase(tx, gr, br, ur, c)
	gUC := NewGoalUsecase(gr, br, ur)
	return NewMessageUsecase(tx, ur, c, rUC, gUC)
}

func makeUser() *domain.User {
	return &domain.User{ID: uuid.New(), Phone: "+6281234567890", Name: "test"}
}

// --- tests ---

func TestHandle_UnknownPhone_Returns404Sentinel(t *testing.T) {
	uc := newUsecase(t, &fakeUserRepo{user: nil}, &fakeTxRepo{}, &fakeGoalRepo{}, &fakeBudgetRepo{}, &fakeComposer{})
	_, err := uc.Handle(context.Background(), "+62000", "x", "2026-05-10T10:00:00Z")
	if !errors.Is(err, ErrUserNotRegistered) {
		t.Errorf("err: got %v, want ErrUserNotRegistered", err)
	}
}

func TestHandle_LowConfidenceClarifies(t *testing.T) {
	uc := newUsecase(t,
		&fakeUserRepo{user: makeUser()},
		&fakeTxRepo{},
		&fakeGoalRepo{},
		&fakeBudgetRepo{},
		&fakeComposer{parseResult: &domain.ParseResult{Intent: "expense", Amount: 50000, Confidence: 0.5}},
	)
	res, err := uc.Handle(context.Background(), "+6281234567890", "x", "2026-05-10T10:00:00Z")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.ReplyType != "clarify" {
		t.Errorf("reply_type: got %q, want clarify", res.ReplyType)
	}
	if res.Persisted {
		t.Error("should not be persisted")
	}
	if res.ReplyText == "" {
		t.Error("reply_text empty")
	}
}

func TestHandle_UnknownIntentClarifies(t *testing.T) {
	uc := newUsecase(t,
		&fakeUserRepo{user: makeUser()},
		&fakeTxRepo{},
		&fakeGoalRepo{},
		&fakeBudgetRepo{},
		&fakeComposer{parseResult: &domain.ParseResult{Intent: "unknown", Confidence: 0.99}},
	)
	res, _ := uc.Handle(context.Background(), "+6281234567890", "x", "2026-05-10T10:00:00Z")
	if res.ReplyType != "clarify" {
		t.Errorf("reply_type: got %q", res.ReplyType)
	}
}

func TestHandle_SetGoalWithoutAmountClarifies(t *testing.T) {
	uc := newUsecase(t,
		&fakeUserRepo{user: makeUser()},
		&fakeTxRepo{},
		&fakeGoalRepo{},
		&fakeBudgetRepo{},
		&fakeComposer{parseResult: &domain.ParseResult{Intent: "set_goal", Amount: 0, Confidence: 0.9, GoalName: "laptop"}},
	)
	res, _ := uc.Handle(context.Background(), "+6281234567890", "nabung laptop", "2026-05-10T10:00:00Z")
	if res.ReplyType != "clarify" {
		t.Errorf("reply_type: got %q", res.ReplyType)
	}
}

func TestHandle_ExpensePersistsAndConfirms(t *testing.T) {
	tx := &fakeTxRepo{balance: 2455000}
	uc := newUsecase(t,
		&fakeUserRepo{user: makeUser()},
		tx,
		&fakeGoalRepo{},
		&fakeBudgetRepo{},
		&fakeComposer{
			parseResult: &domain.ParseResult{Intent: "expense", Amount: 45000, Category: "Makan & Minum", Note: "makan", Confidence: 0.95},
			composeText: "✅ Tercatat...",
		},
	)
	res, err := uc.Handle(context.Background(), "+6281234567890", "  makan siang 45rb 🍜  ", "2026-05-10T10:00:00Z")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.ReplyType != "confirm" {
		t.Errorf("reply_type: got %q", res.ReplyType)
	}
	if !res.Persisted {
		t.Error("should be persisted")
	}
	if len(tx.saved) != 1 {
		t.Fatalf("expected 1 saved tx, got %d", len(tx.saved))
	}
	saved := tx.saved[0]
	if saved.RawMessage != "makan siang 45rb 🍜" {
		t.Errorf("raw_message: got %q (should preserve internal whitespace + emoji, trim outer only)", saved.RawMessage)
	}
	if !saved.ReceivedAt.Equal(mustParseRFC3339(t, "2026-05-10T10:00:00Z")) {
		t.Errorf("received_at not propagated")
	}
	if res.Transaction == nil || res.Transaction.Amount != 45000 {
		t.Errorf("transaction view: %+v", res.Transaction)
	}
	if res.Context == nil || res.Context.Balance != 2455000 {
		t.Errorf("context view: %+v", res.Context)
	}
}

func TestHandle_ConfirmFallsBackWhenComposerFails(t *testing.T) {
	tx := &fakeTxRepo{balance: 2455000}
	uc := newUsecase(t,
		&fakeUserRepo{user: makeUser()},
		tx,
		&fakeGoalRepo{},
		&fakeBudgetRepo{},
		&fakeComposer{
			parseResult: &domain.ParseResult{Intent: "expense", Amount: 45000, Category: "Makan & Minum", Confidence: 0.95},
			composeErr:  errors.New("LLM down"),
		},
	)
	res, err := uc.Handle(context.Background(), "+6281234567890", "makan 45rb", "2026-05-10T10:00:00Z")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if res.ReplyType != "confirm" {
		t.Errorf("reply_type: got %q, want confirm (transaction is persisted, composer failure does NOT promote to error)", res.ReplyType)
	}
	if !res.Persisted {
		t.Error("should still be persisted")
	}
	if !strings.Contains(res.ReplyText, "Tercatat") {
		t.Errorf("fallback reply_text missing 'Tercatat': %q", res.ReplyText)
	}
}

func TestHandle_BalanceQueryReturnsInfo(t *testing.T) {
	uc := newUsecase(t,
		&fakeUserRepo{user: makeUser()},
		&fakeTxRepo{balance: 2455000},
		&fakeGoalRepo{},
		&fakeBudgetRepo{},
		&fakeComposer{parseResult: &domain.ParseResult{Intent: "balance", Confidence: 0.99}},
	)
	res, _ := uc.Handle(context.Background(), "+6281234567890", "saldo", "2026-05-10T10:00:00Z")
	if res.ReplyType != "info" {
		t.Errorf("reply_type: got %q", res.ReplyType)
	}
	if res.Data["balance"] != int64(2455000) {
		t.Errorf("data.balance: got %v", res.Data["balance"])
	}
	if !strings.Contains(res.ReplyText, "2.455.000") {
		t.Errorf("reply_text missing balance: %q", res.ReplyText)
	}
}

func TestHandle_DBErrorReturnsError(t *testing.T) {
	uc := newUsecase(t,
		&fakeUserRepo{user: makeUser()},
		&fakeTxRepo{saveErr: errors.New("DB down")},
		&fakeGoalRepo{},
		&fakeBudgetRepo{},
		&fakeComposer{parseResult: &domain.ParseResult{Intent: "expense", Amount: 45000, Category: "Makan", Confidence: 0.95}},
	)
	res, err := uc.Handle(context.Background(), "+6281234567890", "makan 45rb", "2026-05-10T10:00:00Z")
	if err != nil {
		t.Fatalf("err should be swallowed (logged, then reply_type=error): %v", err)
	}
	if res.ReplyType != "error" {
		t.Errorf("reply_type: got %q, want error", res.ReplyType)
	}
	if res.Persisted {
		t.Error("DB error -> not persisted")
	}
	if !strings.Contains(res.ReplyText, "gangguan") {
		t.Errorf("error reply_text: %q", res.ReplyText)
	}
}

func TestHandle_ReportReturnsClarifyStub(t *testing.T) {
	uc := newUsecase(t,
		&fakeUserRepo{user: makeUser()},
		&fakeTxRepo{},
		&fakeGoalRepo{},
		&fakeBudgetRepo{},
		&fakeComposer{parseResult: &domain.ParseResult{Intent: "report", Confidence: 0.9}},
	)
	res, _ := uc.Handle(context.Background(), "+6281234567890", "laporan", "2026-05-10T10:00:00Z")
	if res.ReplyType != "clarify" {
		t.Errorf("reply_type: got %q", res.ReplyType)
	}
	if !strings.Contains(res.ReplyText, "belum tersedia") {
		t.Errorf("reply_text: %q", res.ReplyText)
	}
}

func mustParseRFC3339(t *testing.T, s string) time.Time {
	t.Helper()
	v, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t.Fatalf("parse %q: %v", s, err)
	}
	return v
}
