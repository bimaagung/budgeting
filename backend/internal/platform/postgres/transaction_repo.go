package postgres

import (
	"context"
	"errors"
	"time"

	"budgeting/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type transactionRepo struct{ db *gorm.DB }

func NewTransactionRepository(db *gorm.DB) domain.TransactionRepository {
	return &transactionRepo{db}
}

func (r *transactionRepo) Save(ctx context.Context, tx domain.Transaction) error {
	return r.db.WithContext(ctx).Create(&tx).Error
}

func (r *transactionRepo) DeleteLast(ctx context.Context, userID uuid.UUID) (*domain.Transaction, error) {
	var tx domain.Transaction
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		First(&tx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &tx, r.db.WithContext(ctx).Delete(&tx).Error
}

func (r *transactionRepo) GetBalance(ctx context.Context, userID uuid.UUID) (int64, error) {
	var balance int64
	err := r.db.WithContext(ctx).
		Model(&domain.Transaction{}).
		Select("COALESCE(SUM(CASE WHEN type='income' THEN amount ELSE -amount END), 0)").
		Where("user_id = ?", userID).
		Scan(&balance).Error
	return balance, err
}

func (r *transactionRepo) GetTodaySpendByCategory(ctx context.Context, userID uuid.UUID) (map[string]int64, error) {
	today := time.Now().Format("2006-01-02")
	return r.spendByCategory(ctx, userID, "date::date = ?", today)
}

func (r *transactionRepo) GetMonthSpendByCategory(ctx context.Context, userID uuid.UUID, year, month int) (map[string]int64, error) {
	return r.spendByCategory(ctx, userID,
		"EXTRACT(YEAR FROM date) = ? AND EXTRACT(MONTH FROM date) = ?", year, month)
}

type categoryRow struct {
	Category string
	Total    int64
}

func (r *transactionRepo) spendByCategory(ctx context.Context, userID uuid.UUID, dateCondition string, dateArgs ...any) (map[string]int64, error) {
	args := append([]any{userID}, dateArgs...)
	var rows []categoryRow
	err := r.db.WithContext(ctx).
		Model(&domain.Transaction{}).
		Select("category, SUM(amount) as total").
		Where("user_id = ? AND type = 'expense' AND "+dateCondition, args...).
		Group("category").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	result := make(map[string]int64, len(rows))
	for _, row := range rows {
		result[row.Category] = row.Total
	}
	return result, nil
}
