package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type SavingsGoal struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey"`
	UserID       uuid.UUID  `gorm:"type:uuid;not null;index"`
	Name         string     `gorm:"not null"`
	TargetAmount int64      `gorm:"not null"`
	SavedAmount  int64      `gorm:"not null;default:0"`
	Deadline     *time.Time `gorm:"default:null"`
	IsActive     bool       `gorm:"not null;default:true"`
	CreatedAt    time.Time  `gorm:"autoCreateTime"`
}

type SavingsGoalRepository interface {
	Create(ctx context.Context, goal SavingsGoal) error
	GetActiveByUser(ctx context.Context, userID uuid.UUID) ([]SavingsGoal, error)
}
