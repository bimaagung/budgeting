package domain

import (
	"context"

	"github.com/google/uuid"
)

type BudgetTarget struct {
	ID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID   uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_budget_unique"`
	Category string    `gorm:"not null;uniqueIndex:idx_budget_unique"`
	Amount   int64     `gorm:"not null"`
	Month    int       `gorm:"not null;uniqueIndex:idx_budget_unique"`
	Year     int       `gorm:"not null;uniqueIndex:idx_budget_unique"`
}

type BudgetTargetRepository interface {
	Upsert(ctx context.Context, bt BudgetTarget) error
	GetByUserAndMonth(ctx context.Context, userID uuid.UUID, year, month int) ([]BudgetTarget, error)
}
