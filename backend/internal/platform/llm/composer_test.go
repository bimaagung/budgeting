package llm

import (
	"context"
	"errors"
	"testing"

	"budgeting/internal/domain"
)

type stubProvider struct {
	out           string
	err           error
	gotSystem     string
	gotUser       string
	gotExpectJSON bool
}

func (s *stubProvider) Generate(ctx context.Context, sys, user string, expectJSON bool) (string, error) {
	s.gotSystem = sys
	s.gotUser = user
	s.gotExpectJSON = expectJSON
	return s.out, s.err
}

func TestComposer_ParseMessage_ValidJSON(t *testing.T) {
	stub := &stubProvider{
		out: `{"intent":"expense","amount":45000,"category":"Makan & Minum","note":"makan siang","goal_name":"","confidence":0.95}`,
	}
	c := NewMessageComposer(stub)
	got, err := c.ParseMessage(context.Background(), "makan siang 45rb")
	if err != nil {
		t.Fatalf("ParseMessage err: %v", err)
	}
	if got.Intent != "expense" {
		t.Errorf("intent: got %q, want %q", got.Intent, "expense")
	}
	if got.Amount != 45000 {
		t.Errorf("amount: got %d, want %d", got.Amount, 45000)
	}
	if !stub.gotExpectJSON {
		t.Errorf("expected expectJSON=true for ParseMessage call")
	}
	if stub.gotSystem != parseSystemPrompt {
		t.Errorf("expected parseSystemPrompt to be used")
	}
}

func TestComposer_ParseMessage_MalformedDowngrades(t *testing.T) {
	stub := &stubProvider{out: `not valid json`}
	c := NewMessageComposer(stub)
	got, err := c.ParseMessage(context.Background(), "garbage")
	if err != nil {
		t.Fatalf("ParseMessage err: %v", err)
	}
	if got.Intent != "unknown" {
		t.Errorf("intent: got %q, want %q (downgrade on malformed)", got.Intent, "unknown")
	}
	if got.Confidence != 0 {
		t.Errorf("confidence: got %f, want 0", got.Confidence)
	}
}

func TestComposer_ParseMessage_ProviderError(t *testing.T) {
	stub := &stubProvider{err: errors.New("api down")}
	c := NewMessageComposer(stub)
	_, err := c.ParseMessage(context.Background(), "anything")
	if err == nil {
		t.Fatal("expected provider error to propagate")
	}
}

func TestComposer_FormatPostTransaction_UsesReminderPrompt(t *testing.T) {
	stub := &stubProvider{out: "✅ Tercatat: Rp 45.000"}
	c := NewMessageComposer(stub)
	rc := domain.ReminderContext{Balance: 100000, LastCategory: "Makan & Minum", LastAmount: 45000}
	out, err := c.FormatPostTransaction(context.Background(), rc)
	if err != nil {
		t.Fatalf("FormatPostTransaction err: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if stub.gotSystem != reminderSystemPrompt {
		t.Errorf("expected reminderSystemPrompt to be used")
	}
	if stub.gotExpectJSON {
		t.Errorf("expectJSON should be false for reminder formatting")
	}
}

func TestComposer_FormatDailyReminder_UsesReminderPrompt(t *testing.T) {
	stub := &stubProvider{out: "📅 Ringkasan hari ini..."}
	c := NewMessageComposer(stub)
	rc := domain.ReminderContext{Balance: 100000}
	out, err := c.FormatDailyReminder(context.Background(), rc)
	if err != nil {
		t.Fatalf("FormatDailyReminder err: %v", err)
	}
	if out == "" {
		t.Fatal("expected non-empty output")
	}
	if stub.gotSystem != reminderSystemPrompt {
		t.Errorf("expected reminderSystemPrompt to be used")
	}
	if stub.gotExpectJSON {
		t.Errorf("expectJSON should be false for reminder formatting")
	}
}
