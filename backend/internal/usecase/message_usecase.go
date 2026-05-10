package usecase

import (
	"context"
	"errors"

	"budgeting/internal/domain"
)

// ErrUserNotRegistered is returned when phone has no matching user.
// Handler maps this to HTTP 404. No auto-create per spec Decision 7.
var ErrUserNotRegistered = errors.New("user not registered")

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
	_ = user
	// TODO(Task 5.3): implement full dispatch
	return &MessageResult{ReplyType: "info", Persisted: false, ReplyText: "stub"}, nil
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
