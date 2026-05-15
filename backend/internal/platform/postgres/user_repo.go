package postgres

import (
	"context"
	"errors"

	"budgeting/internal/domain"

	"gorm.io/gorm"
)

type userRepo struct{ db *gorm.DB }

func NewUserRepository(db *gorm.DB) domain.UserRepository {
	return &userRepo{db}
}

func (r *userRepo) FindByPhone(ctx context.Context, phone string) (*domain.User, error) {
	var u domain.User
	err := r.db.WithContext(ctx).Where("phone = ?", phone).First(&u).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	return &u, err
}

func (r *userRepo) Upsert(ctx context.Context, user domain.User) (*domain.User, error) {
	result := r.db.WithContext(ctx).
		Where(domain.User{Phone: user.Phone}).
		FirstOrCreate(&user)
	if result.Error != nil {
		return nil, result.Error
	}
	return &user, nil
}

func (r *userRepo) GetAllPhones(ctx context.Context) ([]string, error) {
	var phones []string
	if err := r.db.WithContext(ctx).Model(&domain.User{}).Pluck("phone", &phones).Error; err != nil {
		return nil, err
	}
	if phones == nil {
		phones = []string{}
	}
	return phones, nil
}
