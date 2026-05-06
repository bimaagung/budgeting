package llm_test

import (
	"context"
	"testing"

	"budgeting/internal/platform/llm"
)

func TestParseMessage(t *testing.T) {
	composer := llm.NewMessageComposer()
	ctx := context.Background()

	tests := []struct {
		input          string
		wantIntent     string
		wantMinAmount  int64
		wantCategory   string
	}{
		{"makan siang sama temen 45rb", "expense", 45000, "Makan & Minum"},
		{"makan 45000", "expense", 45000, "Makan & Minum"},
		{"mkn siang 45k", "expense", 45000, "Makan & Minum"},
		{"50rb makan", "expense", 50000, "Makan & Minum"},
		{"bakso 15ribu", "expense", 15000, "Makan & Minum"},
		{"grab ke kantor 25k", "expense", 25000, "Transport"},
		{"bensin 50rb", "expense", 50000, "Transport"},
		{"netflix 54rb", "expense", 54000, "Hiburan"},
		{"gaji bulan ini 5jt", "income", 5000000, "Gaji"},
		{"dapat freelance 500rb", "income", 500000, "Freelance"},
		{"saldo berapa", "balance", 0, ""},
		{"sisa uang gue", "balance", 0, ""},
		{"hapus yang tadi", "delete_last", 0, ""},
		{"mau nabung buat laptop 10 juta", "set_goal", 10000000, ""},
		{"budget makan bulan ini 500rb", "set_budget", 500000, "Makan & Minum"},
		{"nabung udah berapa", "check_goal", 0, ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result, err := composer.ParseMessage(ctx, tt.input)
			if err != nil {
				t.Fatalf("ParseMessage error: %v", err)
			}
			if result.Intent != tt.wantIntent {
				t.Errorf("intent: got %q, want %q", result.Intent, tt.wantIntent)
			}
			if tt.wantMinAmount > 0 && result.Amount != tt.wantMinAmount {
				t.Errorf("amount: got %d, want %d", result.Amount, tt.wantMinAmount)
			}
			if tt.wantCategory != "" && result.Category != tt.wantCategory {
				t.Errorf("category: got %q, want %q", result.Category, tt.wantCategory)
			}
			if result.Confidence < 0.75 {
				t.Logf("low confidence %.2f for: %q", result.Confidence, tt.input)
			}
		})
	}
}
