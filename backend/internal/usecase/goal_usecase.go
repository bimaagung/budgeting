package usecase

import (
	"context"
	"fmt"
	"time"

	"budgeting/internal/domain"
	"budgeting/pkg/currency"

	"github.com/google/uuid"
)

type GoalUsecase struct {
	goalRepo   domain.SavingsGoalRepository
	budgetRepo domain.BudgetTargetRepository
	userRepo   domain.UserRepository
}

func NewGoalUsecase(
	goalRepo domain.SavingsGoalRepository,
	budgetRepo domain.BudgetTargetRepository,
	userRepo domain.UserRepository,
) *GoalUsecase {
	return &GoalUsecase{goalRepo, budgetRepo, userRepo}
}

func (u *GoalUsecase) SetGoal(ctx context.Context, user *domain.User, name string, targetAmount int64) (string, error) {
	goal := domain.SavingsGoal{
		ID:           uuid.New(),
		UserID:       user.ID,
		Name:         name,
		TargetAmount: targetAmount,
		IsActive:     true,
		CreatedAt:    time.Now(),
	}
	if err := u.goalRepo.Create(ctx, goal); err != nil {
		return "", err
	}
	return fmt.Sprintf("🎯 Target *%s* Rp %s sudah dicatat! Semangat nabung ya 💪", name, currency.FormatIDR(targetAmount)), nil
}

func (u *GoalUsecase) SetBudget(ctx context.Context, user *domain.User, cat string, amount int64) (string, error) {
	now := time.Now()
	bt := domain.BudgetTarget{
		ID:       uuid.New(),
		UserID:   user.ID,
		Category: cat,
		Amount:   amount,
		Month:    int(now.Month()),
		Year:     now.Year(),
	}
	if err := u.budgetRepo.Upsert(ctx, bt); err != nil {
		return "", err
	}
	return fmt.Sprintf("✅ Budget *%s* bulan ini diset *Rp %s*", cat, currency.FormatIDR(amount)), nil
}

func (u *GoalUsecase) ResolveUser(ctx context.Context, phone string) (*domain.User, error) {
	return u.userRepo.FindByPhone(ctx, phone)
}

func (u *GoalUsecase) CheckGoals(ctx context.Context, user *domain.User) (string, error) {
	goals, err := u.goalRepo.GetActiveByUser(ctx, user.ID)
	if err != nil {
		return "", err
	}
	if len(goals) == 0 {
		return "Belum ada target tabungan aktif. Coba: *nabung buat laptop 10 juta*", nil
	}

	reply := "🎯 *Progress Tabungan Kamu:*\n\n"
	for _, g := range goals {
		pct := int64(0)
		if g.TargetAmount > 0 {
			pct = g.SavedAmount * 100 / g.TargetAmount
		}
		reply += fmt.Sprintf("• *%s*\n  Target: Rp %s\n  Terkumpul: Rp %s (%d%%)\n\n",
			g.Name, currency.FormatIDR(g.TargetAmount), currency.FormatIDR(g.SavedAmount), pct)
	}
	return reply, nil
}
