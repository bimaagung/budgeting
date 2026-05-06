package postgres

import (
	"context"

	"budgeting/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type savingsGoalRepo struct{ db *gorm.DB }

func NewSavingsGoalRepository(db *gorm.DB) domain.SavingsGoalRepository {
	return &savingsGoalRepo{db}
}

func (r *savingsGoalRepo) Create(ctx context.Context, goal domain.SavingsGoal) error {
	return r.db.WithContext(ctx).Create(&goal).Error
}

func (r *savingsGoalRepo) GetActiveByUser(ctx context.Context, userID uuid.UUID) ([]domain.SavingsGoal, error) {
	var goals []domain.SavingsGoal
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND is_active = true", userID).
		Order("created_at").
		Find(&goals).Error
	return goals, err
}
