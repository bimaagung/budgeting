package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Transaction struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:transactions_dedupe,priority:1"`
	Type       string    `gorm:"not null"`
	Amount     int64     `gorm:"not null"`
	Category   string    `gorm:"not null"`
	Note       string    `gorm:"not null;default:''"`
	Date       time.Time `gorm:"not null;default:now()"`
	ReceivedAt time.Time `gorm:"not null;default:now();uniqueIndex:transactions_dedupe,priority:2"`
	CreatedAt  time.Time `gorm:"autoCreateTime"`
	RawMessage string    `gorm:"not null;default:''"`
}

type TransactionRepository interface {
	Save(ctx context.Context, tx *Transaction) error
	DeleteLast(ctx context.Context, userID uuid.UUID) (*Transaction, error)
	GetBalance(ctx context.Context, userID uuid.UUID) (int64, error)
	GetTodaySpendByCategory(ctx context.Context, userID uuid.UUID) (map[string]int64, error)
	GetMonthSpendByCategory(ctx context.Context, userID uuid.UUID, year, month int) (map[string]int64, error)
}
