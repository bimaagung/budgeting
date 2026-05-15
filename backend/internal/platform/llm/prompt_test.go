package llm

import (
	"strings"
	"testing"

	"budgeting/internal/domain"
)

func TestBuildPostTransactionPrompt_ContainsPERINGATANWhenAlertTrue(t *testing.T) {
	rc := domain.ReminderContext{
		LastCategory:  "Makan & Minum",
		LastAmount:    50_000,
		Balance:       2_000_000,
		SpendingAlert: true,
	}
	out := buildPostTransactionPrompt(rc)
	if !strings.Contains(out, "PERINGATAN") {
		t.Errorf("output should contain PERINGATAN when SpendingAlert=true, got:\n%s", out)
	}
}

func TestBuildPostTransactionPrompt_NoPERINGATANWhenAlertFalse(t *testing.T) {
	rc := domain.ReminderContext{
		LastCategory:  "Makan & Minum",
		LastAmount:    50_000,
		Balance:       2_000_000,
		SpendingAlert: false,
	}
	out := buildPostTransactionPrompt(rc)
	if strings.Contains(out, "PERINGATAN") {
		t.Errorf("output should NOT contain PERINGATAN when SpendingAlert=false, got:\n%s", out)
	}
}
