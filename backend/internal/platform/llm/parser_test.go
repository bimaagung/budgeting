package llm_test

import (
	"context"
	"os"
	"testing"

	"budgeting/internal/platform/llm"
)

// TestParseMessage_LiveAPI exercises Claude API end-to-end.
// Skipped unless ANTHROPIC_API_KEY is set; intended for nightly CI, not every PR.
func TestParseMessage_LiveAPI(t *testing.T) {
	if os.Getenv("ANTHROPIC_API_KEY") == "" {
		t.Skip("ANTHROPIC_API_KEY not set; live LLM test skipped")
	}

	composer := llm.NewMessageComposer()
	ctx := context.Background()

	tests := []struct {
		input        string
		wantIntent   string
		wantMinAmt   int64
		wantCategory string
	}{
		{"makan siang sama temen 45rb", "expense", 45000, "Makan & Minum"},
		{"grab ke kantor 25k", "expense", 25000, "Transport"},
		{"gaji bulan ini 5jt", "income", 5000000, "Gaji"},
		{"saldo berapa", "balance", 0, ""},
		{"hapus yang tadi", "delete_last", 0, ""},
		{"mau nabung buat laptop 10 juta", "set_goal", 10000000, ""},
		{"budget makan bulan ini 500rb", "set_budget", 500000, "Makan & Minum"},
		{"nabung udah berapa", "check_goal", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := composer.ParseMessage(ctx, tt.input)
			if err != nil {
				t.Fatalf("ParseMessage err: %v", err)
			}
			if got.Intent != tt.wantIntent {
				t.Errorf("intent: got %q, want %q", got.Intent, tt.wantIntent)
			}
			if tt.wantMinAmt > 0 && got.Amount != tt.wantMinAmt {
				t.Errorf("amount: got %d, want %d", got.Amount, tt.wantMinAmt)
			}
			if tt.wantCategory != "" && got.Category != tt.wantCategory {
				t.Errorf("category: got %q, want %q", got.Category, tt.wantCategory)
			}
		})
	}
}
