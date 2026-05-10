package usecase

import (
	"context"
	"errors"
	"log"
	"strings"
	"time"

	"budgeting/internal/domain"

	"github.com/google/uuid"
)

// ErrUserNotRegistered is returned when phone has no matching user.
// Handler maps this to HTTP 404. No auto-create per spec Decision 7.
var ErrUserNotRegistered = errors.New("user not registered")

const confidenceThreshold = 0.75

type MessageUsecase struct {
	txRepo   domain.TransactionRepository
	userRepo domain.UserRepository
	composer domain.MessageComposer
	reminder *ReminderUsecase
	goal     *GoalUsecase
}

func NewMessageUsecase(
	txRepo domain.TransactionRepository,
	userRepo domain.UserRepository,
	composer domain.MessageComposer,
	reminder *ReminderUsecase,
	goal *GoalUsecase,
) *MessageUsecase {
	return &MessageUsecase{txRepo, userRepo, composer, reminder, goal}
}

// MessageResult is the envelope returned by Handle. Maps 1:1 to handler.Response;
// handler converts to JSON DTOs at the wire.
type MessageResult struct {
	ReplyType   string // "confirm" | "clarify" | "info" | "error"
	Persisted   bool
	ReplyText   string
	Transaction *MessageTransaction
	Context     *MessageContext
	Data        map[string]any
}

type MessageTransaction struct {
	ID       string
	Amount   int64
	Category string
	Type     string
}

type MessageContext struct {
	Balance        int64
	CategoryBudget *MessageCategoryBudget
	SavingsGoals   []MessageSavingsGoal
}

type MessageCategoryBudget struct {
	Category  string
	Spent     int64
	Limit     int64
	Remaining int64
}

type MessageSavingsGoal struct {
	Name        string
	Saved       int64
	Target      int64
	ProgressPct int
}

func (u *MessageUsecase) Handle(ctx context.Context, phone, rawMessage, receivedAt string) (*MessageResult, error) {
	user, err := u.findUser(ctx, phone)
	if err != nil {
		return nil, err
	}

	rcv, err := time.Parse(time.RFC3339, receivedAt)
	if err != nil {
		// Handler validates this; reaching here indicates a wiring bug.
		log.Printf("usecase received invalid received_at %q: %v", receivedAt, err)
		return errorResult(), nil
	}

	trimmed := strings.TrimSpace(rawMessage)

	parsed, err := u.composer.ParseMessage(ctx, trimmed)
	if err != nil {
		log.Printf("composer.ParseMessage error: %v", err)
		return errorResult(), nil
	}

	if parsed.Confidence < confidenceThreshold {
		return clarifyResult(replyClarifyLowConfidence()), nil
	}
	if parsed.Intent == "unknown" {
		return clarifyResult(replyClarifyUnknown()), nil
	}

	switch parsed.Intent {
	case "expense", "income":
		return u.handleTransaction(ctx, user, parsed, trimmed, rcv)
	case "balance":
		return u.handleBalance(ctx, user)
	case "delete_last":
		return u.handleDeleteLast(ctx, user)
	case "set_goal":
		return u.handleSetGoal(ctx, user, parsed)
	case "set_budget":
		return u.handleSetBudget(ctx, user, parsed)
	case "check_goal":
		return u.handleCheckGoal(ctx, user)
	case "report":
		return clarifyResult(replyReportNotAvailable()), nil
	default:
		return clarifyResult(replyClarifyUnknown()), nil
	}
}

func (u *MessageUsecase) handleTransaction(
	ctx context.Context, user *domain.User, parsed *domain.ParseResult,
	rawMessage string, rcv time.Time,
) (*MessageResult, error) {
	tx := domain.Transaction{
		ID:         uuid.New(),
		UserID:     user.ID,
		Type:       parsed.Intent,
		Amount:     parsed.Amount,
		Category:   parsed.Category,
		Note:       parsed.Note,
		Date:       time.Now(),
		ReceivedAt: rcv,
		RawMessage: rawMessage,
	}
	if err := u.txRepo.Save(ctx, &tx); err != nil {
		log.Printf("transaction save failed: %v", err)
		return errorResult(), nil
	}

	rc, err := u.reminder.BuildContext(ctx, user, parsed.Category, parsed.Amount)
	if err != nil {
		log.Printf("build context failed: %v", err)
		return errorResult(), nil
	}

	composed, composeErr := u.composer.FormatPostTransaction(ctx, *rc)
	if composeErr != nil {
		log.Printf("composer.FormatPostTransaction failed: %v", composeErr)
		composed = replyConfirmFallback(parsed.Intent, parsed.Category, parsed.Amount, rc.Balance)
	}

	return &MessageResult{
		ReplyType: "confirm",
		Persisted: true,
		ReplyText: composed,
		Transaction: &MessageTransaction{
			ID: tx.ID.String(), Amount: tx.Amount, Category: tx.Category, Type: tx.Type,
		},
		Context: buildMessageContext(rc),
	}, nil
}

func buildMessageContext(rc *domain.ReminderContext) *MessageContext {
	mc := &MessageContext{
		Balance:      rc.Balance,
		SavingsGoals: make([]MessageSavingsGoal, 0, len(rc.SavingsGoals)),
	}
	if rc.LastCategory != "" {
		if remaining, ok := rc.BudgetRemaining[rc.LastCategory]; ok {
			mc.CategoryBudget = &MessageCategoryBudget{
				Category:  rc.LastCategory,
				Spent:     0, // month spend not yet exposed on ReminderContext; follow-up task
				Limit:     remaining,
				Remaining: remaining,
			}
		}
	}
	for _, g := range rc.SavingsGoals {
		pct := 0
		if g.TargetAmount > 0 {
			pct = int(g.SavedAmount * 100 / g.TargetAmount)
		}
		mc.SavingsGoals = append(mc.SavingsGoals, MessageSavingsGoal{
			Name: g.Name, Saved: g.SavedAmount, Target: g.TargetAmount, ProgressPct: pct,
		})
	}
	return mc
}

func (u *MessageUsecase) handleBalance(ctx context.Context, user *domain.User) (*MessageResult, error) {
	balance, err := u.txRepo.GetBalance(ctx, user.ID)
	if err != nil {
		log.Printf("get balance failed: %v", err)
		return errorResult(), nil
	}
	return &MessageResult{
		ReplyType: "info",
		Persisted: false,
		ReplyText: replyBalance(balance),
		Data:      map[string]any{"balance": balance},
	}, nil
}

func (u *MessageUsecase) handleDeleteLast(ctx context.Context, user *domain.User) (*MessageResult, error) {
	tx, err := u.txRepo.DeleteLast(ctx, user.ID)
	if err != nil {
		log.Printf("delete last failed: %v", err)
		return errorResult(), nil
	}
	if tx == nil {
		return &MessageResult{ReplyType: "info", Persisted: false, ReplyText: replyDeleteLastEmpty()}, nil
	}
	return &MessageResult{ReplyType: "info", Persisted: false, ReplyText: replyDeleteLastSuccess(tx.Note, tx.Amount)}, nil
}

func (u *MessageUsecase) handleSetGoal(ctx context.Context, user *domain.User, parsed *domain.ParseResult) (*MessageResult, error) {
	if parsed.Amount == 0 {
		return clarifyResult(replyClarifyMissingAmount("set_goal")), nil
	}
	name := parsed.GoalName
	if name == "" {
		name = parsed.Note
	}
	reply, err := u.goal.SetGoal(ctx, user, name, parsed.Amount)
	if err != nil {
		log.Printf("set goal failed: %v", err)
		return errorResult(), nil
	}
	return &MessageResult{ReplyType: "info", Persisted: false, ReplyText: reply}, nil
}

func (u *MessageUsecase) handleSetBudget(ctx context.Context, user *domain.User, parsed *domain.ParseResult) (*MessageResult, error) {
	if parsed.Amount == 0 {
		return clarifyResult(replyClarifyMissingAmount("set_budget")), nil
	}
	reply, err := u.goal.SetBudget(ctx, user, parsed.Category, parsed.Amount)
	if err != nil {
		log.Printf("set budget failed: %v", err)
		return errorResult(), nil
	}
	return &MessageResult{ReplyType: "info", Persisted: false, ReplyText: reply}, nil
}

func (u *MessageUsecase) handleCheckGoal(ctx context.Context, user *domain.User) (*MessageResult, error) {
	reply, err := u.goal.CheckGoals(ctx, user)
	if err != nil {
		log.Printf("check goal failed: %v", err)
		return errorResult(), nil
	}
	return &MessageResult{ReplyType: "info", Persisted: false, ReplyText: reply}, nil
}

func (u *MessageUsecase) findUser(ctx context.Context, phone string) (*domain.User, error) {
	user, err := u.userRepo.FindByPhone(ctx, phone)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, ErrUserNotRegistered
	}
	return user, nil
}

func errorResult() *MessageResult {
	return &MessageResult{ReplyType: "error", Persisted: false, ReplyText: replyError()}
}

func clarifyResult(text string) *MessageResult {
	return &MessageResult{ReplyType: "clarify", Persisted: false, ReplyText: text}
}
