package usecase

import (
	"context"
	"time"

	"budgeting/internal/domain"
)

type ReminderUsecase struct {
	txRepo     domain.TransactionRepository
	goalRepo   domain.SavingsGoalRepository
	budgetRepo domain.BudgetTargetRepository
	userRepo   domain.UserRepository
	composer   domain.MessageComposer
}

func NewReminderUsecase(
	txRepo domain.TransactionRepository,
	goalRepo domain.SavingsGoalRepository,
	budgetRepo domain.BudgetTargetRepository,
	userRepo domain.UserRepository,
	composer domain.MessageComposer,
) *ReminderUsecase {
	return &ReminderUsecase{txRepo, goalRepo, budgetRepo, userRepo, composer}
}

func (u *ReminderUsecase) BuildDailyReminder(ctx context.Context, phone string) (string, error) {
	user, err := u.userRepo.FindByPhone(ctx, phone)
	if err != nil || user == nil {
		return "Pengguna tidak ditemukan.", nil
	}
	rc, err := u.BuildContext(ctx, user, "", 0)
	if err != nil {
		return "", err
	}
	return u.composer.FormatDailyReminder(ctx, *rc)
}

func (u *ReminderUsecase) BuildContext(ctx context.Context, user *domain.User, lastCategory string, lastAmount int64) (*domain.ReminderContext, error) {
	now := time.Now()

	balance, err := u.txRepo.GetBalance(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	todayByCategory, err := u.txRepo.GetTodaySpendByCategory(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	monthByCategory, err := u.txRepo.GetMonthSpendByCategory(ctx, user.ID, now.Year(), int(now.Month()))
	if err != nil {
		return nil, err
	}

	budgets, err := u.budgetRepo.GetByUserAndMonth(ctx, user.ID, now.Year(), int(now.Month()))
	if err != nil {
		return nil, err
	}

	goals, err := u.goalRepo.GetActiveByUser(ctx, user.ID)
	if err != nil {
		return nil, err
	}

	budgetRemaining := make(map[string]int64)
	for _, b := range budgets {
		budgetRemaining[b.Category] = b.Amount - monthByCategory[b.Category]
	}

	var todayTotal int64
	for _, v := range todayByCategory {
		todayTotal += v
	}

	return &domain.ReminderContext{
		UserName:        user.Name,
		Balance:         balance,
		LastCategory:    lastCategory,
		LastAmount:      lastAmount,
		TodaySpend:      todayTotal,
		SpendByCategory: todayByCategory,
		BudgetRemaining: budgetRemaining,
		SavingsGoals:    goals,
	}, nil
}
