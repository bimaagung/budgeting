package usecase

import (
	"strings"
	"testing"
)

func TestReplyClarifyLowConfidence(t *testing.T) {
	got := replyClarifyLowConfidence()
	if !strings.Contains(got, "kurang yakin") {
		t.Errorf("low-confidence clarify should mention uncertainty: %q", got)
	}
}

func TestReplyClarifyUnknown(t *testing.T) {
	got := replyClarifyUnknown()
	if got == "" {
		t.Error("clarify-unknown text empty")
	}
}

func TestReplyClarifyMissingAmount(t *testing.T) {
	got := replyClarifyMissingAmount("set_goal")
	if !strings.Contains(got, "nominal") {
		t.Errorf("missing-amount clarify should ask for nominal: %q", got)
	}
}

func TestReplyReportNotAvailable(t *testing.T) {
	got := replyReportNotAvailable()
	if !strings.Contains(got, "belum tersedia") {
		t.Errorf("report stub: %q", got)
	}
}

func TestReplyError(t *testing.T) {
	got := replyError()
	if !strings.Contains(got, "gangguan") {
		t.Errorf("error fallback should mention 'gangguan': %q", got)
	}
}

func TestReplyConfirmFallback(t *testing.T) {
	got := replyConfirmFallback("expense", "Makan & Minum", 45000, 2455000)
	for _, want := range []string{"Tercatat", "Makan & Minum", "45.000", "2.455.000"} {
		if !strings.Contains(got, want) {
			t.Errorf("fallback missing %q: %q", want, got)
		}
	}
}

func TestReplyBalance(t *testing.T) {
	got := replyBalance(2455000)
	if !strings.Contains(got, "2.455.000") {
		t.Errorf("balance reply: %q", got)
	}
}

func TestReplyDeleteLast(t *testing.T) {
	got := replyDeleteLastSuccess("nasi goreng", 25000)
	if !strings.Contains(got, "nasi goreng") || !strings.Contains(got, "25.000") {
		t.Errorf("delete reply: %q", got)
	}
	got = replyDeleteLastEmpty()
	if !strings.Contains(got, "Tidak ada") {
		t.Errorf("delete-empty reply: %q", got)
	}
}

func TestReplyCheckGoal(t *testing.T) {
	goals := []MessageSavingsGoal{
		{Name: "Laptop", Saved: 3200000, Target: 10000000, ProgressPct: 32},
	}
	got := replyCheckGoals(goals)
	for _, want := range []string{"Laptop", "32%"} {
		if !strings.Contains(got, want) {
			t.Errorf("check-goal missing %q: %q", want, got)
		}
	}
	got = replyCheckGoals(nil)
	if !strings.Contains(got, "Belum ada") {
		t.Errorf("empty goals: %q", got)
	}
}
