package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"budgeting/internal/usecase"

	"github.com/gofiber/fiber/v2"
)

// fakeUsecase implements just enough of the usecase contract for handler-level tests.
type fakeUsecase struct {
	calledWithPhone   string
	calledWithMessage string
	calledWithRcvAt   string
	returnErr         error
	returnResult      *usecase.MessageResult
}

func (f *fakeUsecase) Handle(ctx context.Context, phone, message, receivedAt string) (*usecase.MessageResult, error) {
	f.calledWithPhone = phone
	f.calledWithMessage = message
	f.calledWithRcvAt = receivedAt
	return f.returnResult, f.returnErr
}

func newApp(uc messageUsecase) *fiber.App {
	app := fiber.New()
	h := &MessageHandler{uc: uc}
	app.Post("/api/message", h.Handle)
	return app
}

func post(app *fiber.App, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest("POST", "/api/message", bytes.NewReader([]byte(body)))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	resp, _ := app.Test(req, -1)
	rec.Code = resp.StatusCode
	defer resp.Body.Close()
	buf := new(bytes.Buffer)
	_, _ = buf.ReadFrom(resp.Body)
	rec.Body = buf
	return rec
}

func TestHandle_RejectsMissingFields(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{"missing phone", `{"message":"x","received_at":"2026-05-10T10:00:00Z"}`, "phone is required"},
		{"missing message", `{"phone":"+6281234567890","received_at":"2026-05-10T10:00:00Z"}`, "message is required"},
		{"missing received_at", `{"phone":"+6281234567890","message":"x"}`, "received_at is required"},
		{"empty phone", `{"phone":"","message":"x","received_at":"2026-05-10T10:00:00Z"}`, "phone is required"},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			fake := &fakeUsecase{}
			app := newApp(fake)
			rec := post(app, tt.body)
			if rec.Code != 400 {
				t.Errorf("status: got %d, want 400", rec.Code)
			}
			if !strings.Contains(rec.Body.String(), tt.want) {
				t.Errorf("body %q does not contain %q", rec.Body.String(), tt.want)
			}
			if fake.calledWithPhone != "" {
				t.Error("usecase should not be called when validation fails")
			}
		})
	}
}

func TestHandle_RejectsInvalidReceivedAt(t *testing.T) {
	fake := &fakeUsecase{}
	app := newApp(fake)
	rec := post(app, `{"phone":"+6281234567890","message":"x","received_at":"yesterday"}`)
	if rec.Code != 400 {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "received_at must be RFC3339") {
		t.Errorf("body: %s", rec.Body.String())
	}
}

func TestHandle_RejectsInvalidPhone(t *testing.T) {
	fake := &fakeUsecase{}
	app := newApp(fake)
	rec := post(app, `{"phone":"abc","message":"x","received_at":"2026-05-10T10:00:00Z"}`)
	if rec.Code != 400 {
		t.Errorf("status: got %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "invalid phone format") {
		t.Errorf("body: %s", rec.Body.String())
	}
}

func TestHandle_NormalizesPhoneBeforeUsecase(t *testing.T) {
	fake := &fakeUsecase{
		returnResult: &usecase.MessageResult{Reply: "ok", NeedsConfirm: false},
	}
	app := newApp(fake)
	rec := post(app, `{"phone":"081234567890","message":"x","received_at":"2026-05-10T10:00:00Z"}`)
	if rec.Code != 200 {
		t.Fatalf("status: got %d, body: %s", rec.Code, rec.Body.String())
	}
	if fake.calledWithPhone != "+6281234567890" {
		t.Errorf("phone passed to usecase: got %q, want %q", fake.calledWithPhone, "+6281234567890")
	}
}

func TestHandle_404WhenUserNotRegistered(t *testing.T) {
	fake := &fakeUsecase{returnErr: usecase.ErrUserNotRegistered}
	app := newApp(fake)
	rec := post(app, `{"phone":"+6281234567890","message":"x","received_at":"2026-05-10T10:00:00Z"}`)
	if rec.Code != 404 {
		t.Errorf("status: got %d, want 404", rec.Code)
	}
	body := rec.Body.String()
	var parsed map[string]string
	if err := json.Unmarshal([]byte(body), &parsed); err != nil {
		t.Fatalf("body not JSON: %s", body)
	}
	if parsed["error"] != "user not registered" {
		t.Errorf("error field: got %q", parsed["error"])
	}
}
