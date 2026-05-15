package postgres

import (
	"context"
	"errors"
	"time"

	"budgeting/internal/domain"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
)

type transactionRepo struct{ db *gorm.DB }

func NewTransactionRepository(db *gorm.DB) domain.TransactionRepository {
	return &transactionRepo{db}
}

// Save persists a transaction. If a row with the same (user_id, received_at)
// already exists (unique-violation 23505), Save mutates tx to match the
// existing row and returns nil — caller treats this as idempotent success.
func (r *transactionRepo) Save(ctx context.Context, tx *domain.Transaction) error {
	err := r.db.WithContext(ctx).Create(tx).Error
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "transactions_dedupe" {
		var existing domain.Transaction
		fetchErr := r.db.WithContext(ctx).
			Where("user_id = ? AND received_at = ?", tx.UserID, tx.ReceivedAt).
			First(&existing).Error
		if fetchErr != nil {
			return fetchErr
		}
		*tx = existing
		return nil
	}
	return err
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

func (r *transactionRepo) GetMonthIncome(ctx context.Context, userID uuid.UUID, year, month int) (int64, error) {
	var total int64
	err := r.db.WithContext(ctx).
		Model(&domain.Transaction{}).
		Select("COALESCE(SUM(amount), 0)").
		Where("user_id = ? AND type = 'income' AND EXTRACT(YEAR FROM date) = ? AND EXTRACT(MONTH FROM date) = ?",
			userID, year, month).
		Scan(&total).Error
	return total, err
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
