package usecase

import (
	"context"
	"fmt"
	"time"

	"budgeting/internal/domain"
	"budgeting/pkg/currency"

	"github.com/google/uuid"
)

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

type MessageResult struct {
	Reply        string
	NeedsConfirm bool
}

func (u *MessageUsecase) Handle(ctx context.Context, phone, rawMessage string) (*MessageResult, error) {
	user, err := u.getOrCreateUser(ctx, phone)
	if err != nil {
		return nil, err
	}

	parsed, err := u.composer.ParseMessage(ctx, rawMessage)
	if err != nil {
		return nil, err
	}

	if parsed.Confidence < 0.75 {
		return &MessageResult{
			Reply:        "Maaf, saya kurang yakin maksudnya 🤔\nBisa tulis ulang? Contoh:\n• *makan 50rb*\n• *gaji 5jt*\n• *nabung laptop 10jt*",
			NeedsConfirm: true,
		}, nil
	}

	switch parsed.Intent {
	case "expense", "income":
		reply, err := u.recordTransaction(ctx, user, parsed, rawMessage)
		return &MessageResult{Reply: reply}, err

	case "balance":
		reply, err := u.getBalance(ctx, user)
		return &MessageResult{Reply: reply}, err

	case "delete_last":
		reply, err := u.deleteLast(ctx, user)
		return &MessageResult{Reply: reply}, err

	case "set_goal":
		name := parsed.GoalName
		if name == "" {
			name = parsed.Note
		}
		reply, err := u.goal.SetGoal(ctx, user, name, parsed.Amount)
		return &MessageResult{Reply: reply}, err

	case "set_budget":
		reply, err := u.goal.SetBudget(ctx, user, parsed.Category, parsed.Amount)
		return &MessageResult{Reply: reply}, err

	case "check_goal":
		reply, err := u.goal.CheckGoals(ctx, user)
		return &MessageResult{Reply: reply}, err

	default:
		return &MessageResult{Reply: "Saya belum bisa membantu dengan itu.\nCoba: *makan 50rb*, *gaji 5jt*, atau *saldo berapa?*"}, nil
	}
}

func (u *MessageUsecase) recordTransaction(ctx context.Context, user *domain.User, parsed *domain.ParseResult, raw string) (string, error) {
	tx := domain.Transaction{
		ID:         uuid.New(),
		UserID:     user.ID,
		Type:       parsed.Intent,
		Amount:     parsed.Amount,
		Category:   parsed.Category,
		Note:       parsed.Note,
		Date:       time.Now(),
		RawMessage: raw,
	}
	if err := u.txRepo.Save(ctx, tx); err != nil {
		return "", err
	}

	rc, err := u.reminder.BuildContext(ctx, user, parsed.Category, parsed.Amount)
	if err != nil {
		return "", err
	}

	return u.composer.FormatPostTransaction(ctx, *rc)
}

func (u *MessageUsecase) deleteLast(ctx context.Context, user *domain.User) (string, error) {
	tx, err := u.txRepo.DeleteLast(ctx, user.ID)
	if err != nil {
		return "Tidak ada transaksi yang bisa dihapus.", nil
	}
	return fmt.Sprintf("🗑️ Transaksi dihapus: _%s_ Rp %s", tx.Note, currency.FormatIDR(tx.Amount)), nil
}

func (u *MessageUsecase) getBalance(ctx context.Context, user *domain.User) (string, error) {
	balance, err := u.txRepo.GetBalance(ctx, user.ID)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("💰 Saldo kamu saat ini: *Rp %s*", currency.FormatIDR(balance)), nil
}

func (u *MessageUsecase) getOrCreateUser(ctx context.Context, phone string) (*domain.User, error) {
	user, err := u.userRepo.FindByPhone(ctx, phone)
	if err != nil {
		return nil, err
	}
	if user != nil {
		return user, nil
	}
	return u.userRepo.Upsert(ctx, domain.User{ID: uuid.New(), Phone: phone, Name: phone})
}
