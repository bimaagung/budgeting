package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"testing"

	"github.com/gofiber/fiber/v2"
)

type fakePhoneQuerier struct {
	phones []string
	err    error
}

func (f *fakePhoneQuerier) GetAllPhones(ctx context.Context) ([]string, error) {
	return f.phones, f.err
}

func newUserApp(q userPhoneQuerier) *fiber.App {
	app := fiber.New()
	h := &UserHandler{q: q}
	app.Get("/api/users/phones", h.Handle)
	return app
}

func TestUserHandler_ReturnsPhonesAsJSON(t *testing.T) {
	q := &fakePhoneQuerier{phones: []string{"+62811111111", "+62822222222"}}
	app := newUserApp(q)

	req := httptest.NewRequest("GET", "/api/users/phones", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var body struct {
		Phones []string `json:"phones"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Phones) != 2 {
		t.Errorf("phones count: got %d, want 2", len(body.Phones))
	}
	if body.Phones[0] != "+62811111111" {
		t.Errorf("phones[0]: got %q, want %q", body.Phones[0], "+62811111111")
	}
}

func TestUserHandler_EmptyPhones_ReturnsEmptyArray(t *testing.T) {
	q := &fakePhoneQuerier{phones: []string{}}
	app := newUserApp(q)

	req := httptest.NewRequest("GET", "/api/users/phones", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("status: got %d, want 200", resp.StatusCode)
	}

	var body struct {
		Phones []string `json:"phones"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if body.Phones == nil {
		t.Error("phones must be [] not null")
	}
}

func TestUserHandler_DBError_Returns500(t *testing.T) {
	q := &fakePhoneQuerier{err: errors.New("db down")}
	app := newUserApp(q)

	req := httptest.NewRequest("GET", "/api/users/phones", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test: %v", err)
	}
	if resp.StatusCode != 500 {
		t.Fatalf("status: got %d, want 500", resp.StatusCode)
	}
}
