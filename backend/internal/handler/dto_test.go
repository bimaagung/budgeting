package handler

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestResponse_ConfirmShape(t *testing.T) {
	resp := Response{
		ReplyType: "confirm",
		Persisted: true,
		ReplyText: "ok",
		Transaction: &TransactionView{
			ID: "abc", Amount: 45000, Category: "Makan & Minum", Type: "expense",
		},
		Context: &ContextView{
			Balance:        2455000,
			CategoryBudget: nil, // null in JSON
			SavingsGoals:   []SavingsGoalView{},
		},
	}
	b, err := json.Marshal(resp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	// encoding/json HTML-escapes & as &; normalize for readable assertions.
	s := strings.ReplaceAll(string(b), "\\u0026", "&")
	for _, want := range []string{
		`"reply_type":"confirm"`,
		`"persisted":true`,
		`"reply_text":"ok"`,
		`"transaction":{"id":"abc","amount":45000,"category":"Makan & Minum","type":"expense"}`,
		`"category_budget":null`,
		`"savings_goals":[]`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output %q missing %q", s, want)
		}
	}
	for _, must := range []string{`"data"`} {
		if strings.Contains(s, must) {
			t.Errorf("output should not contain %q (got %q)", must, s)
		}
	}
}

func TestResponse_InfoShape(t *testing.T) {
	resp := Response{
		ReplyType: "info",
		Persisted: false,
		ReplyText: "Saldo Rp 2.4jt",
		Data:      map[string]any{"balance": int64(2455000)},
	}
	b, _ := json.Marshal(resp)
	s := string(b)
	for _, want := range []string{
		`"reply_type":"info"`,
		`"data":{"balance":2455000}`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %q", want, s)
		}
	}
	for _, must := range []string{`"transaction"`, `"context"`} {
		if strings.Contains(s, must) {
			t.Errorf("info response leaked %q: %q", must, s)
		}
	}
}

func TestResponse_ClarifyShape(t *testing.T) {
	resp := Response{ReplyType: "clarify", Persisted: false, ReplyText: "hmm?"}
	b, _ := json.Marshal(resp)
	s := string(b)
	for _, must := range []string{`"transaction"`, `"context"`, `"data"`} {
		if strings.Contains(s, must) {
			t.Errorf("clarify response leaked %q: %q", must, s)
		}
	}
}
