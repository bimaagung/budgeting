package postgres_test

import (
	"context"
	"testing"

	"budgeting/internal/domain"
	"budgeting/internal/platform/postgres"

	"github.com/google/uuid"
)

func TestGetAllPhones_ReturnsSeedPhones(t *testing.T) {
	db, cleanup := newTestDB(t)
	defer cleanup()

	repo := postgres.NewUserRepository(db)

	phones := []string{
		"+62" + uuid.New().String()[:10],
		"+62" + uuid.New().String()[:10],
	}
	for _, p := range phones {
		u := domain.User{ID: uuid.New(), Phone: p, Name: "test"}
		if err := db.Create(&u).Error; err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	got, err := repo.GetAllPhones(context.Background())
	if err != nil {
		t.Fatalf("GetAllPhones: %v", err)
	}
	if len(got) < 2 {
		t.Errorf("expected at least 2 phones, got %d: %v", len(got), got)
	}
	phoneSet := make(map[string]bool)
	for _, p := range got {
		phoneSet[p] = true
	}
	for _, want := range phones {
		if !phoneSet[want] {
			t.Errorf("phone %q not found in result %v", want, got)
		}
	}
}

// Empty-table case (nil → []string{}) is covered by TestUserHandler_EmptyPhones_ReturnsEmptyArray
// in handler test. Not tested here to avoid DELETE FROM users violating FK constraints.
