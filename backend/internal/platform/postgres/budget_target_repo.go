package postgres

import (
	"context"

	"budgeting/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm/clause"
	"gorm.io/gorm"
)

type budgetTargetRepo struct{ db *gorm.DB }

func NewBudgetTargetRepository(db *gorm.DB) domain.BudgetTargetRepository {
	return &budgetTargetRepo{db}
}

func (r *budgetTargetRepo) Upsert(ctx context.Context, bt domain.BudgetTarget) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "user_id"}, {Name: "category"}, {Name: "month"}, {Name: "year"}},
			DoUpdates: clause.AssignmentColumns([]string{"amount"}),
		}).
		Create(&bt).Error
}

func (r *budgetTargetRepo) GetByUserAndMonth(ctx context.Context, userID uuid.UUID, year, month int) ([]domain.BudgetTarget, error) {
	var targets []domain.BudgetTarget
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND year = ? AND month = ?", userID, year, month).
		Find(&targets).Error
	return targets, err
}
