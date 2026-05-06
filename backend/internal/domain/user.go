package domain

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey"`
	Phone     string    `gorm:"uniqueIndex;not null"`
	Name      string    `gorm:"not null;default:''"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
}

type UserRepository interface {
	FindByPhone(ctx context.Context, phone string) (*User, error)
	Upsert(ctx context.Context, user User) (*User, error)
}
